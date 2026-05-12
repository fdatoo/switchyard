# Live entity state and Devices controls — implementation plan

> **For agentic workers:** Use superpowers:executing-plans to work through this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Wire `EntityService.Subscribe` end-to-end so entity state is live across the UI, and add per-capability controls (toggle, brightness, color temp, color picker) to the Devices driver-detail rail.

**Architecture:** New `rpcStream()` primitive does Connect server-streaming over raw fetch (manual envelope parsing — keeps the `@connectrpc/connect` dep out). A singleton `entityStore` hydrates from `ListEntities + ListDevices` then opens `Subscribe` for live updates with exponential reconnect. New per-capability widgets (`SyEntityToggle`, `SyBrightnessSlider`, etc.) are dumb presentational; `SyEntityRow` orchestrates optimistic mutations through `callCapability`. `SyDriverPanel` gains an entities section; `HomeView` reads its count from the store.

**Tech Stack:** Vue 3 (composition API, `<script setup>`), TypeScript, Vite, raw fetch (no `@connectrpc/connect`), Playwright for validation.

**Verification strategy (no vitest in `app/`):** each task ends in `vue-tsc -b --noEmit` typecheck + a targeted Playwright probe against the running daemon (`http://localhost:5174` proxied to UDS at `~/.local/share/switchyard/switchyardd.sock`). The hue_main driver is registered with real entities — perfect for end-to-end validation.

**Reference spec:** `docs/design/specs/2026-05-11-live-entity-state-and-controls-design.md`

---

## Task 1: Streaming RPC primitive (`rpcStream`)

**Files:**
- Modify: `app/src/data/rpc.ts`

The Connect server-streaming wire format is a sequence of envelopes:
`[1 byte flags][4 bytes length BE][N bytes JSON payload]`. The trailer
envelope sets `flags & 0x02` (`FlagEndStream`); its payload is JSON
`{"error?": {...}, "metadata?": {...}}`. The request body is a single
envelope wrapping the request message (flags = 0).

- [ ] **Step 1: Add `rpcStream` implementation**

Add at the bottom of `app/src/data/rpc.ts`:

```ts
const STREAM_PROTOCOL_CT = "application/connect+json";
const FLAG_END_STREAM = 0x02;

/**
 * Call a Connect server-streaming RPC. Returns an async generator that
 * yields each `TItem` as it arrives. The trailer envelope ends the
 * stream; if it carries an error, it's thrown as `RpcError`.
 *
 * Aborting `opts.signal` causes the underlying fetch to reject and the
 * generator to throw the abort reason.
 */
export async function* rpcStream<TReq, TItem>(
  serviceMethod: string,
  request: TReq,
  opts: RpcOptions = {},
): AsyncGenerator<TItem, void, void> {
  // Encode the single client message into a length-prefixed envelope.
  const reqBytes = new TextEncoder().encode(JSON.stringify(request ?? {}));
  const envelope = new Uint8Array(5 + reqBytes.length);
  envelope[0] = 0;
  new DataView(envelope.buffer).setUint32(1, reqBytes.length, false);
  envelope.set(reqBytes, 5);

  const res = await fetch(`/${serviceMethod}`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": STREAM_PROTOCOL_CT,
      "Connect-Protocol-Version": PROTOCOL_VERSION,
    },
    body: envelope,
    signal: opts.signal,
  });
  if (!res.ok) {
    let detail = `${res.status} ${res.statusText}`;
    try {
      const text = await res.text();
      if (text) detail = `${detail}: ${text.slice(0, 240)}`;
    } catch { /* ignore */ }
    throw new RpcError(res.status, serviceMethod, detail);
  }
  if (!res.body) {
    throw new RpcError(0, serviceMethod, "stream response missing body");
  }

  const reader = res.body.getReader();
  let buffer = new Uint8Array(0);
  const decoder = new TextDecoder();

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (value && value.length) {
        const next = new Uint8Array(buffer.length + value.length);
        next.set(buffer, 0);
        next.set(value, buffer.length);
        buffer = next;
      }
      // Drain whole envelopes from the buffer.
      while (buffer.length >= 5) {
        const flags = buffer[0];
        const len = new DataView(
          buffer.buffer, buffer.byteOffset + 1, 4,
        ).getUint32(0, false);
        if (buffer.length < 5 + len) break;
        const payload = buffer.subarray(5, 5 + len);
        buffer = buffer.subarray(5 + len);
        const text = decoder.decode(payload);
        if (flags & FLAG_END_STREAM) {
          // Trailer. May carry { error: {code, message} }.
          if (text.length > 0) {
            try {
              const parsed = JSON.parse(text) as { error?: { code?: string; message?: string } };
              if (parsed.error) {
                throw new RpcError(
                  0,
                  serviceMethod,
                  `${parsed.error.code ?? "stream_error"}: ${parsed.error.message ?? ""}`,
                );
              }
            } catch (err) {
              if (err instanceof RpcError) throw err;
              // Malformed trailer — treat as benign EOS.
            }
          }
          return;
        }
        yield JSON.parse(text) as TItem;
      }
      if (done) {
        // Stream ended without an explicit trailer envelope. Treat as EOS.
        return;
      }
    }
  } finally {
    try { await reader.cancel(); } catch { /* ignore */ }
  }
}
```

- [ ] **Step 2: Typecheck**

Run: `cd app && npx vue-tsc -b --noEmit`
Expected: clean (no errors).

- [ ] **Step 3: Smoke probe via Playwright**

Open `http://localhost:5174`, evaluate in the page (uses the same proxy):

```js
async () => {
  const mod = await import("/src/data/rpc.ts");
  const it = mod.rpcStream(
    "switchyard.v1alpha1.EntityService/Subscribe",
    { selector: {}, fromCursor: 0 },
  );
  const out = [];
  const iter = it[Symbol.asyncIterator]();
  // Pull 1 message with a 2s timeout (heartbeat or change).
  const t = setTimeout(() => iter.return && iter.return(), 2000);
  try {
    const { value } = await iter.next();
    if (value) out.push(value);
  } finally { clearTimeout(t); }
  return out;
}
```

Expected: a single object — either an `EntityChange` (entity_id, cursor, entity) or a `Heartbeat`. No error thrown.

- [ ] **Step 4: Commit**

```bash
git add app/src/data/rpc.ts
git commit -m "feat(app): rpcStream() Connect server-streaming primitive"
```

---

## Task 2: subscribeEntities + listDevices wrappers

**Files:**
- Modify: `app/src/data/entities.ts`

- [ ] **Step 1: Add subscribe + device types/wrappers**

Append to `app/src/data/entities.ts`:

```ts
import { rpcStream } from "./rpc";

/** Selector subset used by the UI today — matches proto EntitySelector
 *  but only the fields we use are surfaced. */
export interface EntitySelector {
  entityIds?: string[];
  deviceIds?: string[];
  areas?: string[];
  zones?: string[];
  classes?: string[];
}

export interface EntityChange {
  entityId: string;
  cursor: number;
  at?: string;
  entity: Entity;
}
export interface EntityHeartbeat {
  /* present, body unused by store */
}
export type SubscribeMessage =
  | { change: EntityChange; heartbeat?: undefined }
  | { change?: undefined; heartbeat: EntityHeartbeat };

export interface Device {
  id: string;
  friendlyName: string;
  areaId: string;
  driverInstanceId: string;
  entityIds: string[];
}

const ENTITY_SVC = "switchyard.v1alpha1.EntityService";
const DEVICE_SVC = "switchyard.v1alpha1.DeviceService";

/** Server-streaming subscription. Yields each Subscribe message until the
 *  stream ends or `opts.signal` aborts. `fromCursor=0` means live-from-now. */
export function subscribeEntities(
  selector: EntitySelector = {},
  fromCursor = 0,
  opts: RpcOptions = {},
): AsyncGenerator<SubscribeMessage, void, void> {
  return rpcStream<unknown, SubscribeMessage>(
    `${ENTITY_SVC}/Subscribe`,
    { selector, fromCursor },
    opts,
  );
}

/** One-shot device list — used by the entity store to map
 *  device_id → driver_instance_id for byDriver() lookups. */
export async function listDevices(opts: RpcOptions = {}): Promise<{ devices: Device[] }> {
  return rpcCall<unknown, { devices: Device[] }>(
    `${DEVICE_SVC}/List`,
    {},
    opts,
  );
}
```

- [ ] **Step 2: Typecheck**

`cd app && npx vue-tsc -b --noEmit` → clean.

- [ ] **Step 3: Probe `listDevices` via Playwright**

```js
async () => {
  const m = await import("/src/data/entities.ts");
  const r = await m.listDevices();
  return { count: r.devices.length, sample: r.devices[0] ?? null };
}
```

Expected: a count (matches `dist/switchyard device list`) and a sample device with `driverInstanceId` set.

- [ ] **Step 4: Commit**

```bash
git add app/src/data/entities.ts
git commit -m "feat(app): subscribeEntities() stream + listDevices() wrapper"
```

---

## Task 3: callCapability write path

**Files:**
- Create: `app/src/data/call-capability.ts`

- [ ] **Step 1: Create the module**

```ts
/**
 * EntityService.CallCapability client. Translates a parameter object into
 * the proto's google.protobuf.Struct shape (the wire format is just JSON
 * for Struct, so we pass the object through as-is) and surfaces the
 * (success, errorMessage) outcome.
 */

import { rpcCall, type RpcOptions } from "./rpc";

const ENTITY_SVC = "switchyard.v1alpha1.EntityService";

export interface CallCapabilityResult {
  correlationId: string;
  success: boolean;
  errorMessage: string;
}

export async function callCapability(
  entityId: string,
  capability: string,
  parameters: Record<string, unknown> = {},
  opts: RpcOptions = {},
): Promise<CallCapabilityResult> {
  const res = await rpcCall<unknown, CallCapabilityResult>(
    `${ENTITY_SVC}/CallCapability`,
    { entityId, capability, parameters },
    opts,
  );
  return {
    correlationId: res.correlationId ?? "",
    success: !!res.success,
    errorMessage: res.errorMessage ?? "",
  };
}
```

- [ ] **Step 2: Typecheck + probe**

`cd app && npx vue-tsc -b --noEmit` → clean.

Playwright probe: pick the first hue light entity, toggle it. Expects success and the bulb to flip (visually confirm if you're nearby; otherwise check the next state via Subscribe in Task 4).

```js
async () => {
  const e = await import("/src/data/entities.ts");
  const c = await import("/src/data/call-capability.ts");
  const list = await e.listEntities();
  const light = list.entities.find(x => x.type === "light");
  if (!light) return { skipped: true };
  const before = light.state?.light?.on ?? false;
  const r = await c.callCapability(light.id, before ? "turn_off" : "turn_on", {});
  return { id: light.id, before, ...r };
}
```

Expected: `{ success: true, errorMessage: "" }`.

- [ ] **Step 3: Commit**

```bash
git add app/src/data/call-capability.ts
git commit -m "feat(app): callCapability() write path"
```

---

## Task 4: Entity store

**Files:**
- Create: `app/src/stores/entity-store.ts`

- [ ] **Step 1: Create the store module**

```ts
/**
 * Singleton entity store. Hydrates from ListEntities + ListDevices, then
 * opens a Subscribe stream for live updates. Reconnects with exponential
 * backoff on stream errors, replaying from `lastCursor`.
 *
 * Consumers read `entities` (a reactive shallowRef<Map>) and the derived
 * helpers byDriver / byArea. Writes go through applyOptimistic which
 * returns a revert thunk.
 */

import { computed, shallowRef, type ComputedRef, type Ref } from "vue";
import {
  listDevices, listEntities, subscribeEntities,
  type Device, type Entity, type SubscribeMessage,
} from "@/data/entities";

const RECONNECT_BASE_MS = 1_000;
const RECONNECT_MAX_MS = 30_000;
const HEARTBEAT_TIMEOUT_MS = 30_000;

interface EntityStore {
  entities: Readonly<Ref<ReadonlyMap<string, Entity>>>;
  connected: Readonly<Ref<boolean>>;
  hydrated: Readonly<Ref<boolean>>;
  start(): Promise<void>;
  stop(): void;
  byDriver(driverInstanceId: string): ComputedRef<Entity[]>;
  byArea(areaId: string): ComputedRef<Entity[]>;
  applyOptimistic(id: string, patch: Partial<Entity>): () => void;
}

function createStore(): EntityStore {
  const entitiesRef = shallowRef<Map<string, Entity>>(new Map());
  const connected = shallowRef<boolean>(false);
  const hydrated = shallowRef<boolean>(false);
  const deviceToDriver = shallowRef<Map<string, string>>(new Map());

  let abort: AbortController | null = null;
  let started = false;
  let reconnectAttempt = 0;
  let reconnectTimer: number | null = null;
  let watchdog: number | null = null;
  let lastCursor = 0;
  let lastSeenAt = Date.now();
  let optimisticGen = 0;
  // Map<entityId, latestOptimisticGen> — used by revert() to no-op when
  // a real EntityChange has already overwritten the optimistic state.
  const optimisticByEntity = new Map<string, number>();

  function upsert(id: string, e: Entity): void {
    const m = new Map(entitiesRef.value);
    m.set(id, e);
    entitiesRef.value = m;
    // Real change wins over any pending optimistic generation.
    optimisticByEntity.delete(id);
  }

  function rebuildDeviceMap(devices: Device[]): void {
    const m = new Map<string, string>();
    for (const d of devices) {
      m.set(d.id, d.driverInstanceId);
      // Some entities don't have a device (synthetic / driver-level).
      // Drivers expose their entity_ids on Device — we trust that mapping.
      for (const eid of d.entityIds) {
        // Index entityId → driverInstance directly so byDriver doesn't
        // require two map lookups for entities whose device_id is empty.
        m.set(`entity:${eid}`, d.driverInstanceId);
      }
    }
    deviceToDriver.value = m;
  }

  function driverOf(entity: Entity): string | undefined {
    if (entity.deviceId) {
      const v = deviceToDriver.value.get(entity.deviceId);
      if (v) return v;
    }
    return deviceToDriver.value.get(`entity:${entity.id}`);
  }

  async function hydrate(signal: AbortSignal): Promise<void> {
    const [eRes, dRes] = await Promise.all([
      listEntities({ signal }),
      listDevices({ signal }),
    ]);
    rebuildDeviceMap(dRes.devices);
    const m = new Map<string, Entity>();
    for (const e of eRes.entities) m.set(e.id, e);
    entitiesRef.value = m;
    hydrated.value = true;
  }

  function clearReconnect(): void {
    if (reconnectTimer !== null) {
      window.clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  }

  function startWatchdog(): void {
    if (watchdog !== null) window.clearInterval(watchdog);
    watchdog = window.setInterval(() => {
      if (!connected.value) return;
      if (Date.now() - lastSeenAt > HEARTBEAT_TIMEOUT_MS) {
        // Force-close; the catch in runStream will schedule a reconnect.
        abort?.abort();
      }
    }, 5_000);
  }

  function scheduleReconnect(): void {
    clearReconnect();
    const base = Math.min(RECONNECT_BASE_MS * 2 ** reconnectAttempt, RECONNECT_MAX_MS);
    const jitter = base * (0.9 + Math.random() * 0.2);
    reconnectAttempt += 1;
    reconnectTimer = window.setTimeout(() => { void runStream(); }, jitter);
  }

  async function runStream(): Promise<void> {
    if (!started) return;
    abort = new AbortController();
    try {
      // Hydrate on first run only.
      if (!hydrated.value) await hydrate(abort.signal);
      connected.value = true;
      lastSeenAt = Date.now();
      startWatchdog();
      const stream = subscribeEntities({}, lastCursor, { signal: abort.signal });
      for await (const msg of stream) {
        lastSeenAt = Date.now();
        if (msg.change) {
          lastCursor = Math.max(lastCursor, msg.change.cursor);
          upsert(msg.change.entityId, msg.change.entity);
        }
        // Heartbeat: nothing to do — lastSeenAt already bumped.
      }
      // Stream closed cleanly (server-initiated EOS). Treat as a
      // disconnect and reconnect.
      connected.value = false;
      if (started) scheduleReconnect();
    } catch (err) {
      connected.value = false;
      if (!started) return;
      if ((err as Error).name === "AbortError") {
        // Aborted due to stop() — do nothing.
        return;
      }
      scheduleReconnect();
    }
  }

  return {
    entities: entitiesRef,
    connected,
    hydrated,

    async start(): Promise<void> {
      if (started) return;
      started = true;
      reconnectAttempt = 0;
      await runStream();
    },

    stop(): void {
      started = false;
      clearReconnect();
      if (watchdog !== null) {
        window.clearInterval(watchdog);
        watchdog = null;
      }
      abort?.abort();
      abort = null;
      connected.value = false;
    },

    byDriver(driverInstanceId: string): ComputedRef<Entity[]> {
      return computed<Entity[]>(() => {
        const out: Entity[] = [];
        for (const e of entitiesRef.value.values()) {
          if (driverOf(e) === driverInstanceId) out.push(e);
        }
        return out;
      });
    },

    byArea(areaId: string): ComputedRef<Entity[]> {
      return computed<Entity[]>(() => {
        const out: Entity[] = [];
        for (const e of entitiesRef.value.values()) {
          if (e.areaId === areaId) out.push(e);
        }
        return out;
      });
    },

    applyOptimistic(id: string, patch: Partial<Entity>): () => void {
      const cur = entitiesRef.value.get(id);
      if (!cur) return () => { /* no entry to revert */ };
      const gen = ++optimisticGen;
      optimisticByEntity.set(id, gen);
      const merged: Entity = {
        ...cur,
        ...patch,
        state: { ...cur.state, ...patch.state, ...((patch.state as { light?: unknown })?.light ? { light: { ...cur.state?.light, ...(patch.state as { light: object }).light } } : {}) } as Entity["state"],
      };
      const m = new Map(entitiesRef.value);
      m.set(id, merged);
      entitiesRef.value = m;
      return () => {
        if (optimisticByEntity.get(id) !== gen) return; // already overwritten
        optimisticByEntity.delete(id);
        const m2 = new Map(entitiesRef.value);
        m2.set(id, cur);
        entitiesRef.value = m2;
      };
    },
  };
}

export const entityStore: EntityStore = createStore();
```

- [ ] **Step 2: Typecheck**

`cd app && npx vue-tsc -b --noEmit` → clean.

- [ ] **Step 3: Smoke probe**

```js
async () => {
  const { entityStore } = await import("/src/stores/entity-store.ts");
  await entityStore.start();
  // Wait one tick for the initial snapshot.
  await new Promise(r => setTimeout(r, 500));
  const out = {
    hydrated: entityStore.hydrated.value,
    connected: entityStore.connected.value,
    count: entityStore.entities.value.size,
  };
  // Don't stop — leave running for next probes.
  return out;
}
```

Expected: `{ hydrated: true, connected: true, count: >0 }` matching `dist/switchyard entity list | wc -l` (modulo header lines).

- [ ] **Step 4: Commit**

```bash
git add app/src/stores/entity-store.ts
git commit -m "feat(app): entity store — hydrate + Subscribe + reconnect"
```

---

## Task 5: Wire AppLayout to start the store

**Files:**
- Modify: `app/src/views/AppLayout.vue`

- [ ] **Step 1: Import + lifecycle**

In the existing `<script setup>` block, add the import alongside the others:

```ts
import { entityStore } from "@/stores/entity-store";
```

In the `onMounted` block (already exists for the `⌘K` keydown listener), add:

```ts
void entityStore.start();
```

In the `onBeforeUnmount` block, add:

```ts
entityStore.stop();
```

- [ ] **Step 2: Typecheck**

`cd app && npx vue-tsc -b --noEmit` → clean.

- [ ] **Step 3: Reload + verify via Playwright**

Reload `http://localhost:5174`. Evaluate:

```js
async () => {
  const { entityStore } = await import("/src/stores/entity-store.ts");
  // Give it a beat to hydrate.
  await new Promise(r => setTimeout(r, 800));
  return { hydrated: entityStore.hydrated.value, connected: entityStore.connected.value, count: entityStore.entities.value.size };
}
```

Expected: hydrated: true, connected: true, count > 0.

- [ ] **Step 4: Commit**

```bash
git add app/src/views/AppLayout.vue
git commit -m "feat(app): AppLayout wires entity-store start/stop"
```

---

## Task 6: HomeView stat tile uses the store

**Files:**
- Modify: `app/src/views/HomeView.vue`

- [ ] **Step 1: Replace the `loadEntities` block**

Remove the existing entity loader and bind the tile to the store:

- Delete the `entityCount`, `entitiesLoading`, and `loadEntities` declarations.
- Delete the `void loadEntities()` call inside `onMounted`.
- Add at the top of `<script setup>`:
  ```ts
  import { entityStore } from "@/stores/entity-store";
  import { computed } from "vue";
  // ...alongside existing imports.
  ```
- Add (or merge into existing computeds):
  ```ts
  const entityCount = computed<number>(() => entityStore.entities.value.size);
  const entitiesLoading = computed<boolean>(() => !entityStore.hydrated.value);
  ```

The template binding for the Entities `SyStatTile` already references
`:value="entityCount"` and `:loading="entitiesLoading"` — no template
change needed.

- [ ] **Step 2: Typecheck**

`cd app && npx vue-tsc -b --noEmit` → clean.

- [ ] **Step 3: Visual check**

Navigate to `/`, screenshot via Playwright (`home-stat-from-store.png`).
The Entities stat tile should show the count immediately on load (no
flash of "—") and match the prior value.

- [ ] **Step 4: Commit**

```bash
git add app/src/views/HomeView.vue
git commit -m "feat(app): HomeView entities tile reads from entityStore"
```

---

## Task 7: SyEntityToggle widget

**Files:**
- Create: `app/src/lib/components/entity-controls/SyEntityToggle.vue`

- [ ] **Step 1: Component**

```vue
<!--
  SyEntityToggle — on/off switch for light/switch entities.
  Pure presentational: emits change(next), parent owns optimistic state +
  callCapability orchestration.
-->
<script setup lang="ts">
import { SySwitch } from "@/lib";

defineProps<{
  on: boolean;
  busy?: boolean;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "change", next: boolean): void;
}>();

function onUpdate(v: boolean): void {
  emit("change", v);
}
</script>

<template>
  <SySwitch
    :model-value="on"
    :disabled="disabled || busy"
    @update:model-value="onUpdate"
  />
</template>
```

- [ ] **Step 2: Typecheck**

`cd app && npx vue-tsc -b --noEmit` → clean.

- [ ] **Step 3: Commit**

```bash
git add app/src/lib/components/entity-controls/SyEntityToggle.vue
git commit -m "feat(app): SyEntityToggle widget"
```

---

## Task 8: SyBrightnessSlider widget

**Files:**
- Create: `app/src/lib/components/entity-controls/SyBrightnessSlider.vue`

- [ ] **Step 1: Component**

```vue
<!--
  SyBrightnessSlider — 0-100% slider for light brightness.

  Owns its own draft value while the user is dragging. Emits `commit` on
  release; parent issues callCapability("set_brightness", { value: 0..255 }).
  When `userIsDragging` is true, external `value` prop changes are
  ignored — prevents the slider snapping back if a stream EntityChange
  arrives mid-drag.
-->
<script setup lang="ts">
import { ref, watch } from "vue";

const props = defineProps<{
  /** Current brightness, 0..255. */
  value: number;
  busy?: boolean;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "commit", next: number): void;
}>();

/** Local 0..100 draft while dragging. */
const draft = ref<number>(toPercent(props.value));
const dragging = ref<boolean>(false);

function toPercent(v: number): number {
  return Math.round((v / 255) * 100);
}
function fromPercent(p: number): number {
  return Math.round((p / 100) * 255);
}

watch(() => props.value, (v) => {
  if (!dragging.value) draft.value = toPercent(v);
});

function onInput(e: Event): void {
  draft.value = Number((e.target as HTMLInputElement).value);
  dragging.value = true;
}
function onChange(): void {
  emit("commit", fromPercent(draft.value));
  // Release dragging shortly after — gives the stream EntityChange a
  // moment to land before we resync to props.
  window.setTimeout(() => { dragging.value = false; }, 500);
}
</script>

<template>
  <div class="sy-brightness">
    <input
      type="range"
      min="0"
      max="100"
      step="1"
      :value="draft"
      :disabled="disabled || busy"
      @input="onInput"
      @change="onChange"
    />
    <span class="sy-brightness__pct">{{ draft }}%</span>
  </div>
</template>

<style scoped>
.sy-brightness {
  display: flex; align-items: center; gap: var(--sy-space-2);
}
.sy-brightness input { flex: 1; }
.sy-brightness__pct {
  font-variant-numeric: tabular-nums;
  font-size: var(--sy-font-size-caption);
  color: var(--sy-color-fg-subtle);
  min-width: 3ch; text-align: right;
}
</style>
```

- [ ] **Step 2: Typecheck**

`cd app && npx vue-tsc -b --noEmit` → clean.

- [ ] **Step 3: Commit**

```bash
git add app/src/lib/components/entity-controls/SyBrightnessSlider.vue
git commit -m "feat(app): SyBrightnessSlider widget"
```

---

## Task 9: SyColorTempSlider widget

**Files:**
- Create: `app/src/lib/components/entity-controls/SyColorTempSlider.vue`

- [ ] **Step 1: Component**

Hue color temperature mireds typically range ~153 (cool, ~6500K) to ~500
(warm, ~2000K). We render the slider with the warm side on the right
because that matches the hue app's affordance.

```vue
<!--
  SyColorTempSlider — color temperature in mireds. Same drag-throttling
  pattern as SyBrightnessSlider: local draft while dragging, commit on
  release.
-->
<script setup lang="ts">
import { ref, watch } from "vue";

const props = defineProps<{
  value: number;       // mireds
  min?: number;        // default 153
  max?: number;        // default 500
  busy?: boolean;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "commit", next: number): void;
}>();

const draft = ref<number>(props.value);
const dragging = ref<boolean>(false);

watch(() => props.value, (v) => {
  if (!dragging.value) draft.value = v;
});

function onInput(e: Event): void {
  draft.value = Number((e.target as HTMLInputElement).value);
  dragging.value = true;
}
function onChange(): void {
  emit("commit", draft.value);
  window.setTimeout(() => { dragging.value = false; }, 500);
}
</script>

<template>
  <div class="sy-temp">
    <input
      type="range"
      :min="min ?? 153"
      :max="max ?? 500"
      step="1"
      :value="draft"
      :disabled="disabled || busy"
      @input="onInput"
      @change="onChange"
    />
    <span class="sy-temp__val">{{ draft }} mired</span>
  </div>
</template>

<style scoped>
.sy-temp { display: flex; align-items: center; gap: var(--sy-space-2); }
.sy-temp input { flex: 1; }
.sy-temp__val {
  font-variant-numeric: tabular-nums;
  font-size: var(--sy-font-size-caption);
  color: var(--sy-color-fg-subtle);
  min-width: 7ch; text-align: right;
}
</style>
```

- [ ] **Step 2: Typecheck + commit**

`cd app && npx vue-tsc -b --noEmit` → clean.

```bash
git add app/src/lib/components/entity-controls/SyColorTempSlider.vue
git commit -m "feat(app): SyColorTempSlider widget"
```

---

## Task 10: SyColorPicker widget

**Files:**
- Create: `app/src/lib/components/entity-controls/SyColorPicker.vue`

- [ ] **Step 1: Component**

```vue
<!--
  SyColorPicker — minimal HTML5 color input. The proto stores RGB as
  uint32 0xRRGGBB; we serialize/parse against the standard "#rrggbb"
  hex form HTML5 emits.
-->
<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{
  /** Current color as 0xRRGGBB (uint32). 0 means unset / unsupported. */
  value: number;
  busy?: boolean;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "commit", rgbHex: string): void;
}>();

const hex = computed<string>(() => {
  const n = (props.value | 0) & 0xffffff;
  return "#" + n.toString(16).padStart(6, "0");
});

function onChange(e: Event): void {
  const v = (e.target as HTMLInputElement).value; // "#rrggbb"
  emit("commit", v.startsWith("#") ? v.slice(1) : v);
}
</script>

<template>
  <input
    type="color"
    :value="hex"
    :disabled="disabled || busy"
    @change="onChange"
  />
</template>

<style scoped>
input[type="color"] {
  width: 36px; height: 28px;
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-sm);
  background: transparent;
  padding: 2px;
}
</style>
```

- [ ] **Step 2: Typecheck + commit**

`cd app && npx vue-tsc -b --noEmit` → clean.

```bash
git add app/src/lib/components/entity-controls/SyColorPicker.vue
git commit -m "feat(app): SyColorPicker widget"
```

---

## Task 11: SySensorValue widget

**Files:**
- Create: `app/src/lib/components/entity-controls/SySensorValue.vue`

- [ ] **Step 1: Component**

```vue
<!--
  SySensorValue — read-only display for numeric_sensor / binary_sensor.
-->
<script setup lang="ts">
import { SyText } from "@/lib";

defineProps<{
  /** Either a numeric value (with unit) or a boolean reading. */
  value: number | boolean;
  unit?: string;
}>();
</script>

<template>
  <div class="sy-sensor">
    <template v-if="typeof value === 'number'">
      <SyText as="span" variant="title" weight="semibold">{{ value }}</SyText>
      <SyText v-if="unit" as="span" variant="caption" tone="subtle">{{ unit }}</SyText>
    </template>
    <template v-else>
      <SyText as="span" variant="title" weight="semibold">
        {{ value ? "active" : "idle" }}
      </SyText>
    </template>
  </div>
</template>

<style scoped>
.sy-sensor { display: inline-flex; align-items: baseline; gap: var(--sy-space-1); }
</style>
```

- [ ] **Step 2: Typecheck + commit**

`cd app && npx vue-tsc -b --noEmit` → clean.

```bash
git add app/src/lib/components/entity-controls/SySensorValue.vue
git commit -m "feat(app): SySensorValue read-only widget"
```

---

## Task 12: SyEntityRow

**Files:**
- Create: `app/src/lib/components/entity-row/SyEntityRow.vue`
- Modify: `app/src/lib/index.ts` (export SyEntityRow + the entity-controls)

- [ ] **Step 1: Component**

```vue
<!--
  SyEntityRow — one entity in a list, with appropriate inline controls.

  Owns the optimistic + callCapability orchestration so the per-capability
  widgets stay dumb. Errors surface inline below the header, auto-dismiss
  after 3s.
-->
<script setup lang="ts">
import { computed, ref } from "vue";
import {
  SyText, SyDot, SyBadge, SyIcon,
  SyEntityToggle, SyBrightnessSlider, SyColorTempSlider,
  SyColorPicker, SySensorValue,
} from "@/lib";
import type { Entity } from "@/data/entities";
import { callCapability } from "@/data/call-capability";
import { entityStore } from "@/stores/entity-store";

const props = defineProps<{
  entity: Entity;
}>();

const expanded = ref<boolean>(false);
const inlineError = ref<string>("");
const busy = ref<boolean>(false);
let errorTimer: number | null = null;

function setError(msg: string): void {
  inlineError.value = msg;
  if (errorTimer) window.clearTimeout(errorTimer);
  errorTimer = window.setTimeout(() => { inlineError.value = ""; }, 3_000);
}

const kind = computed<"light" | "switch" | "numeric_sensor" | "binary_sensor" | "unknown">(() => {
  const s = props.entity.state ?? {};
  if (s.light) return "light";
  if (s.switchDevice) return "switch";
  if (s.numericSensor) return "numeric_sensor";
  if (s.binarySensor) return "binary_sensor";
  return "unknown";
});

const available = computed<boolean>(() => !!props.entity.state?.available);

const primaryLine = computed<string>(() => {
  const s = props.entity.state;
  if (kind.value === "light" && s?.light) {
    if (!s.light.on) return "off";
    const pct = Math.round(((s.light.brightness ?? 0) / 255) * 100);
    return `on · ${pct}%`;
  }
  if (kind.value === "switch" && s?.switchDevice) {
    return s.switchDevice.on ? "on" : "off";
  }
  if (kind.value === "numeric_sensor" && s?.numericSensor) {
    return `${s.numericSensor.value} ${s.numericSensor.unit}`;
  }
  if (kind.value === "binary_sensor" && s?.binarySensor) {
    return s.binarySensor.on ? "active" : "idle";
  }
  return "";
});

/** Whether the light supports color_temp (proto: 0 = unsupported). */
const supportsColorTemp = computed<boolean>(() =>
  (props.entity.capabilities?.light?.colorTemp ?? 0) !== 0);
const supportsColor = computed<boolean>(() =>
  (props.entity.capabilities?.light?.colorRgb ?? 0) !== 0);

async function withCall<T>(
  patch: Partial<Entity>,
  capability: string,
  parameters: Record<string, unknown>,
): Promise<void> {
  const revert = entityStore.applyOptimistic(props.entity.id, patch);
  busy.value = true;
  try {
    const r = await callCapability(props.entity.id, capability, parameters);
    if (!r.success) {
      revert();
      setError(r.errorMessage || "Command rejected by driver");
    }
  } catch (err) {
    revert();
    setError(err instanceof Error ? err.message : String(err));
  } finally {
    busy.value = false;
  }
}

function onToggle(next: boolean): void {
  const cap = next ? "turn_on" : "turn_off";
  if (kind.value === "light") {
    void withCall({ state: { ...props.entity.state, light: { ...(props.entity.state?.light ?? {}), on: next } } }, cap, {});
  } else if (kind.value === "switch") {
    void withCall({ state: { ...props.entity.state, switchDevice: { on: next } } }, cap, {});
  }
}
function onBrightness(next: number): void {
  void withCall({ state: { ...props.entity.state, light: { ...(props.entity.state?.light ?? {}), brightness: next, on: true } } }, "set_brightness", { value: String(next) });
}
function onColorTemp(next: number): void {
  void withCall({ state: { ...props.entity.state, light: { ...(props.entity.state?.light ?? {}), colorTemp: next } } }, "set_color_temp", { mireds: String(next) });
}
function onColor(rgbHex: string): void {
  const n = parseInt(rgbHex, 16);
  void withCall({ state: { ...props.entity.state, light: { ...(props.entity.state?.light ?? {}), colorRgb: n } } }, "set_color", { rgb: rgbHex });
}

function toggleExpand(): void {
  // Lights with controls expand on click; sensors don't (nothing to show).
  if (kind.value === "light") expanded.value = !expanded.value;
}
</script>

<template>
  <div class="sy-er" :class="{ 'sy-er--expanded': expanded }">
    <div class="sy-er__head" @click="toggleExpand">
      <SyIcon :name="kind === 'light' ? 'bulb' : kind === 'switch' ? 'plugin' : 'activity'" :size="18" />
      <div class="sy-er__title">
        <SyText variant="body" weight="medium">{{ entity.friendlyName || entity.id }}</SyText>
        <div class="sy-er__sub">
          <SyBadge intent="neutral" size="sm">{{ entity.type }}</SyBadge>
          <SyDot :intent="available ? 'good' : 'subtle'" :title="available ? 'available' : 'unreachable'" />
          <SyText variant="caption" tone="subtle">{{ primaryLine }}</SyText>
        </div>
      </div>

      <div class="sy-er__inline" @click.stop>
        <SyEntityToggle
          v-if="kind === 'light'"
          :on="entity.state?.light?.on ?? false"
          :busy="busy"
          @change="onToggle"
        />
        <SyEntityToggle
          v-else-if="kind === 'switch'"
          :on="entity.state?.switchDevice?.on ?? false"
          :busy="busy"
          @change="onToggle"
        />
        <SySensorValue
          v-else-if="kind === 'numeric_sensor'"
          :value="entity.state?.numericSensor?.value ?? 0"
          :unit="entity.state?.numericSensor?.unit"
        />
        <SySensorValue
          v-else-if="kind === 'binary_sensor'"
          :value="entity.state?.binarySensor?.on ?? false"
        />
      </div>
    </div>

    <SyText v-if="inlineError" variant="caption" tone="bad" class="sy-er__err">
      {{ inlineError }}
    </SyText>

    <div v-if="expanded && kind === 'light'" class="sy-er__controls" @click.stop>
      <SyBrightnessSlider
        :value="entity.state?.light?.brightness ?? 0"
        :busy="busy"
        @commit="onBrightness"
      />
      <SyColorTempSlider
        v-if="supportsColorTemp"
        :value="entity.state?.light?.colorTemp ?? 0"
        :busy="busy"
        @commit="onColorTemp"
      />
      <SyColorPicker
        v-if="supportsColor"
        :value="entity.state?.light?.colorRgb ?? 0"
        :busy="busy"
        @commit="onColor"
      />
    </div>
  </div>
</template>

<style scoped>
.sy-er {
  display: flex; flex-direction: column;
  padding: var(--sy-space-2) var(--sy-space-3);
  gap: var(--sy-space-2);
}
.sy-er + .sy-er { border-top: 1px solid var(--sy-color-line-soft); }
.sy-er__head {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: var(--sy-space-3);
  cursor: default;
}
.sy-er__head:has(+ .sy-er__controls) { cursor: pointer; }
.sy-er__title { min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.sy-er__sub { display: flex; align-items: center; gap: var(--sy-space-2); }
.sy-er__inline { display: flex; align-items: center; gap: var(--sy-space-2); }
.sy-er__controls {
  display: flex; flex-direction: column; gap: var(--sy-space-2);
  padding-top: var(--sy-space-1);
}
.sy-er__err { color: var(--sy-color-fg-bad); }
</style>
```

- [ ] **Step 2: Add exports to `app/src/lib/index.ts`**

Append:

```ts
export { default as SyEntityRow } from "./components/entity-row/SyEntityRow.vue";
export { default as SyEntityToggle } from "./components/entity-controls/SyEntityToggle.vue";
export { default as SyBrightnessSlider } from "./components/entity-controls/SyBrightnessSlider.vue";
export { default as SyColorTempSlider } from "./components/entity-controls/SyColorTempSlider.vue";
export { default as SyColorPicker } from "./components/entity-controls/SyColorPicker.vue";
export { default as SySensorValue } from "./components/entity-controls/SySensorValue.vue";
```

- [ ] **Step 3: Typecheck + commit**

`cd app && npx vue-tsc -b --noEmit` → clean.

```bash
git add app/src/lib/components/entity-row/ app/src/lib/components/entity-controls/ app/src/lib/index.ts
git commit -m "feat(app): SyEntityRow + entity-controls exports"
```

---

## Task 13: SyDriverPanel renders entities section

**Files:**
- Modify: `app/src/lib/components/driver-panel/SyDriverPanel.vue`

- [ ] **Step 1: Add an `entities` prop + section**

In the existing `defineProps`, add:

```ts
entities?: import("@/data/entities").Entity[];
streamConnected?: boolean;
```

In the template, append a new `<section>` after the existing driver
metadata block:

```vue
<section class="sy-driver-panel__entities">
  <div class="sy-driver-panel__sectionHead">
    <SyText variant="title" weight="semibold">Entities</SyText>
    <SyText v-if="streamConnected === false" variant="caption" tone="subtle">
      reconnecting…
    </SyText>
  </div>

  <SyEmptyState
    v-if="!entities || entities.length === 0"
    title="No entities yet"
    description="This driver hasn't registered any entities."
  >
    <template #icon><SyIcon name="bulb" :size="24" /></template>
  </SyEmptyState>

  <div v-else class="sy-driver-panel__entityList">
    <SyEntityRow v-for="e in entities" :key="e.id" :entity="e" />
  </div>
</section>
```

Add to the existing imports:

```ts
import { SyEntityRow, SyEmptyState, SyIcon } from "@/lib";
```

Add scoped styles:

```css
.sy-driver-panel__entities { display: flex; flex-direction: column; gap: var(--sy-space-2); margin-top: var(--sy-space-4); }
.sy-driver-panel__sectionHead { display: flex; align-items: center; justify-content: space-between; }
.sy-driver-panel__entityList { display: flex; flex-direction: column; }
```

- [ ] **Step 2: Typecheck + commit**

`cd app && npx vue-tsc -b --noEmit` → clean.

```bash
git add app/src/lib/components/driver-panel/SyDriverPanel.vue
git commit -m "feat(app): SyDriverPanel renders entities section"
```

---

## Task 14: DevicesView passes entities from store

**Files:**
- Modify: `app/src/views/DevicesView.vue`

- [ ] **Step 1: Wire the rail to the store**

Add imports:

```ts
import { entityStore } from "@/stores/entity-store";
```

Add a computed that resolves to the selected driver's entities:

```ts
const selectedEntities = computed(() => {
  if (!selectedId.value) return [];
  return entityStore.byDriver(selectedId.value).value;
});
```

In the template where `<SyDriverPanel :driver="..." />` is rendered, add:

```vue
:entities="selectedEntities"
:stream-connected="entityStore.connected.value"
```

- [ ] **Step 2: Typecheck**

`cd app && npx vue-tsc -b --noEmit` → clean.

- [ ] **Step 3: End-to-end Playwright**

1. Navigate to `/devices`.
2. Click `hue_main` row to open the rail.
3. Screenshot `devices-rail-with-entities.png`. Expect: list of light rows, each with toggle.
4. Toggle the first light → screenshot `devices-rail-toggled.png`. Verify the row visually flips.
5. Click the row body to expand → screenshot `devices-rail-expanded.png`. Verify brightness slider + (if present) color temp slider + color picker render.
6. Drag the brightness slider to ~30% → release → screenshot. Verify the percent display updates and the "on · X%" line follows.

- [ ] **Step 4: Commit**

```bash
git add app/src/views/DevicesView.vue
git commit -m "feat(app): DevicesView rail shows live entities for selected driver"
```

---

## Task 15: Final validation + cleanup

- [ ] **Step 1: Full typecheck**

`cd app && npx vue-tsc -b --noEmit` → clean.

- [ ] **Step 2: Tour every page via Playwright**

Visit /, /rooms, /activity, /automations, /devices, /settings. Confirm
no console errors beyond the favicon 404. Confirm the entities stat tile
on Home shows the correct count and never flashes "loading" after the
first hydration.

- [ ] **Step 3: Reconnect test**

While the UI is open, restart the daemon. Watch the "reconnecting…" pill
appear in the entities section header (when on Devices with rail open).
Once the daemon is back, the pill should disappear and the entity rows
should reflect any state changes that happened during the outage.

- [ ] **Step 4: Verify no stray PNG / probe artifacts**

`git status` — should be clean (the .gitignore patterns set up earlier
should suppress any new screenshots / probe scripts). If anything snuck
in, add the pattern.

---

## Self-review notes

- **Spec coverage:** every spec section has a corresponding task — rpcStream (1), subscribeEntities + listDevices (2), callCapability (3), entity-store with hydration + reconnect + applyOptimistic (4), AppLayout wiring (5), HomeView tile (6), all five widgets (7-11), SyEntityRow (12), SyDriverPanel update (13), DevicesView wiring (14), end-to-end validation (15).
- **No placeholders:** every code step shows the actual code. The Playwright probes are written out as snippets.
- **Type consistency:** field names match the proto camelCase (Connect's TS clients camelCase wire fields by default — `friendlyName`, `driverInstanceId`, `colorTemp`, `colorRgb`, `switchDevice`). If the daemon's actual wire shape differs (e.g. snake_case via raw JSON), Task 4's smoke probe will surface it before later tasks build on it.
- **Verification strategy:** typecheck + Playwright probes per task. No vitest setup (deferred — would be its own PR).
