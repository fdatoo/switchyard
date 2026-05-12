# Live entity state and Devices controls — design

**Status:** Draft
**Date:** 2026-05-11
**Branch:** `feat/new-ui`

## Problem

The new Vue UI in `app/` lists entities as static counts and surfaces driver
metadata in the Devices rail, but never reflects what entities are *doing*.
You cannot see whether a light is on, what brightness it is at, what a
sensor reads, or change any of it. Until live state and write paths exist,
Switchyard's UI is a config viewer, not a control surface.

## Goal

Wire `EntityService.Subscribe` end-to-end so the UI tracks live entity state
across the whole installation, and extend the Devices driver-detail rail
with per-entity rows that render the appropriate controls (toggle,
brightness, color temp, color picker) for the entity's capabilities.

In scope:

- A streaming RPC primitive for the Vue client (currently unary-only).
- A singleton entity store fed by an initial `ListEntities()` snapshot
  followed by a single `Subscribe()` for live updates, started at app boot.
- A device → driver-instance map (one `ListDevices()` call) so
  `byDriver(id)` can resolve client-side, since neither `Entity` nor
  `EntitySelector` carry `driver_instance_id` over the wire.
- A `callCapability()` write path with optimistic UI.
- Per-capability control widgets covering Light, Switch, NumericSensor,
  BinarySensor (the four `Attributes.kind` variants the proto defines).
- An "Entities" section appended to `SyDriverPanel` that lists the selected
  driver's entities with the full control set.
- Replacing the one-shot `listEntities` call on the Home stat tile with
  `entityStore.size`.

Out of scope (explicitly deferred):

- Toast / snackbar component — surface action errors inline on the row.
- Lab specimens for the new components (a small follow-up PR).
- Rooms drill-down and Home Quick Controls — both will consume the same
  store; they are separate iterations.
- Capability discovery for non-hue drivers — we hardcode the four hue
  capabilities (`turn_on`, `turn_off`, `set_brightness`, `set_color_temp`,
  `set_color`). A generic capability registry is its own design.

## Architecture

```
┌──────────────────────────────────────────────┐
│ AppLayout (mounted at app boot)              │
│   ├─ entityStore.start() ── opens Subscribe  │
│   └─ entityStore.stop()  ── on unmount       │
└──────────────────────────────────────────────┘
                 │ reactive Map<id, Entity>
                 ▼
   ┌──────────────────────┬─────────────────────┐
   │ HomeView stat tile   │ DevicesView rail    │
   │ entityStore.size     │ store.byDriver(id)  │
   └──────────────────────┴─────────────────────┘
                 ▲
                 │ EntityChange / Heartbeat stream
                 │
   ┌──────────────────────────────────────────┐
   │ rpc.ts: rpcStream() — ConnectRPC server  │
   │ stream w/ AbortController + reconnect    │
   └──────────────────────────────────────────┘
```

Data flow on a user interaction (toggle a light):

1. User clicks `SyEntityToggle` → component calls `entityStore.applyOptimistic(id, patch)` then `callCapability(id, "turn_on", {})` in the background.
2. Daemon dispatches the command to the hue driver, which calls the bridge,
   which emits an SSE event → the driver pushes a `StateChanged` event.
3. The `Subscribe` stream surfaces an `EntityChange` carrying the new full
   `Entity` payload; the store overwrites its entry with the authoritative
   value, replacing the optimistic patch.
4. If the RPC errors, the store reverts the optimistic patch and the row
   surfaces a small inline error.

## Components

### Data layer (`app/src/data/`)

**`rpc.ts`** — add `rpcStream<TReq, TItem>(method, req, opts) → AsyncGenerator<TItem>`.

- Wraps ConnectRPC server-streaming. Yields each `TItem` as it arrives.
- Returns/throws cleanly when `opts.signal` aborts; surfaces network errors
  with the same `RpcError` shape the unary path uses.
- Caller is responsible for `for await ... catch` and the abort lifecycle.

**`entities.ts`** — add `subscribeEntities(selector?, fromCursor?, opts)`.

- Thin wrapper around `rpcStream`. Returns `AsyncGenerator<EntityChange | Heartbeat>`.
- Heartbeats are passed through so the consumer can update a "last seen"
  liveness timestamp. The store uses this to detect a silent stream stall.

**`call-capability.ts`** *(new)* — `callCapability(entityId, capability, params: Record<string, unknown>)`.

- Unary wrapper. Translates the `params` object into `google.protobuf.Struct`.
- Returns `{ success, errorMessage }`. Throws on transport errors.

### Store (`app/src/stores/entity-store.ts`) *(new dir)*

Singleton (module-level instance) using a plain Vue composable + module
state. (No Pinia — the repo doesn't use it today and a single store
doesn't justify the dependency. Revisit if a second store appears.)

Exposes:

- `entities: Readonly<Map<string, Entity>>` — reactive via a `shallowRef`
  containing a `Map`, replaced on every mutation so consumers'
  `computed()` re-runs.
- `connected: Readonly<Ref<boolean>>` — true when the live stream is open.
- `hydrated: Readonly<Ref<boolean>>` — true once the initial snapshot has
  populated the store. Consumers gate "loading" UI on this, not on
  `connected` (we want to keep showing the last-known state during a
  reconnect, not flash to "loading").
- `start(): Promise<void>` — fetch initial snapshot via `ListEntities()`
  and `ListDevices()`, then open `Subscribe()`. Idempotent.
- `stop(): void` — abort all in-flight RPCs and the stream. Idempotent.
- `byDriver(driverInstanceId: string): ComputedRef<Entity[]>` — uses the
  internal device map: `entity.device_id → Device.driver_instance_id`.
- `byArea(areaId: string): ComputedRef<Entity[]>` — filters on `entity.area_id`.
- `applyOptimistic(id: string, patch: Partial<Entity>): () => void` — applies
  a local mutation and returns a `revert` thunk for the caller to invoke on
  RPC error. The store records the optimistic generation; if a real
  `EntityChange` arrives between optimistic-apply and revert, the revert is
  a no-op (the truth has already arrived).

Lifecycle:

- `start()`:
  1. Fetch `listEntities()` and `listDevices()` in parallel; populate the
     entity Map and the device → driver-instance map.
  2. Set `hydrated = true`.
  3. Open `subscribeEntities(selector?, fromCursor)` with
     `fromCursor = lastCursor` (initially 0 = live from now). The
     snapshot from step 1 covers everything before that point.
- On each `EntityChange`, upserts the entity by id and stores `cursor` as `lastCursor`.
- On heartbeat, updates `lastSeenAt`.
- On stream error or close, marks `connected = false` and schedules
  reconnect with exponential backoff (1s, 2s, 4s, 8s, 16s, capped 30s,
  jittered ±10%). On reconnect, re-opens `Subscribe` with `lastCursor` so
  we replay anything missed; the snapshot fetch is *not* repeated unless
  `lastCursor == 0` (i.e., we never successfully connected).
- A 30s watchdog: if `Date.now() - lastSeenAt > 30000` while `connected = true`, force-close and reconnect.

Device map refresh: the device map only matters for `byDriver()` filtering.
Drivers are added/removed via Pkl config reloads, which are rare. Refresh
the device map (`listDevices()`) on every `start()` and additionally when
`DevicesView.load()` runs (it already polls). Out of scope: streaming
device-list changes.

### Components (`app/src/lib/components/`)

**`entity-row/SyEntityRow.vue`**

- Props: `entity: Entity`.
- Compact form: `[icon] friendly_name [type badge] [availability dot] · primary state line`.
- Primary state line per kind:
  - `light`: `on · {brightness%}` / `off`.
  - `switch`: `on` / `off`.
  - `numeric_sensor`: `{value} {unit}`.
  - `binary_sensor`: `active` / `idle`.
- Trailing inline control where applicable: `SyEntityToggle` for light/switch.
- Click row body → expand. Expanded section renders the per-capability
  control set below the header.
- Inline error: a small `SyText tone="bad"` slot below the header,
  populated by the row when `callCapability` fails (auto-clears after 3s,
  click to dismiss). Errors live on the row, not in a global toast.

**`entity-controls/`** *(new directory)* — small focused widgets:

- **`SyEntityToggle.vue`** — toggle switch; props `{ on: boolean, busy: boolean, disabled: boolean }`; emits `change(next: boolean)`.
- **`SyBrightnessSlider.vue`** — range slider 0–100%; props `{ value: number, busy: boolean }`; emits `input(next: number)` while dragging, `commit(next: number)` on release. Maps 0–100 to the proto's 0–255 internally.
- **`SyColorTempSlider.vue`** — range slider, mireds. Hidden by parent when entity has no color_temp capability (parent inspects `entity.capabilities.light.color_temp != 0`).
- **`SyColorPicker.vue`** — minimal HTML5 `<input type="color">` for v1; emits `commit(rgbHex: string)`.
- **`SySensorValue.vue`** — read-only display: large value, small unit beneath.

The control widgets are purely presentational. They emit changes; the parent (`SyEntityRow`) owns the optimistic-apply + `callCapability` orchestration.

### View updates

**`app/src/lib/components/driver-panel/SyDriverPanel.vue`** *(update)*

- Append a section "Entities" below the existing driver metadata.
- Renders `<SyEntityRow v-for>` from a prop `entities: Entity[]` passed in.
- Empty state: `SyEmptyState` with "This driver hasn't registered any entities yet."

**`app/src/views/DevicesView.vue`** *(update)*

- Import `entityStore`. When the rail is open for `selectedId`, pass
  `store.byDriver(selectedId).value` to `SyDriverPanel`.

**`app/src/views/AppLayout.vue`** *(update)*

- `onMounted(() => void entityStore.start())`, `onBeforeUnmount(() => entityStore.stop())`.
- Topbar daemon-status indicator stays untouched. Stream-connection state
  surfaces only as a small inline "reconnecting…" pill in `SyDriverPanel`'s
  entity section header when `!entityStore.connected`. Keeps blast radius
  off the shell.

**`app/src/views/HomeView.vue`** *(update)*

- Replace the one-shot `listEntities` call with `entityStore.entities.size`.
- Loading state on the stat tile is bound to `entityStore.hydrated` (false
  → spinner, true → number). After hydration the tile never reverts to
  "loading" — a stream disconnect leaves the last-known count visible,
  which is the right behaviour: we don't want the dashboard to look broken
  during a transient reconnect.

## Optimistic UI and slider drag throttling

Two distinct cases:

**Discrete actions (toggle, color picker commit):**

1. Snapshot current state.
2. `applyOptimistic` to flip locally → returns `revert`.
3. `callCapability(...)`.
4. On error: call `revert`, set inline error on the row.
5. On success: do nothing — the `EntityChange` will overwrite the optimistic value within milliseconds; if it doesn't arrive within 2s, the optimistic state stands (the daemon already confirmed success).

**Continuous actions (brightness slider, color temp slider):**

- While the user is dragging, the slider component emits `input` events
  (local-only, no RPC). The slider's local value is the source of truth for
  the UI.
- Concurrently, the store may receive `EntityChange` updates for this
  entity. The slider component sets a `userIsDragging` flag and ignores
  external `value` prop changes for that field while it's true.
- On `commit` (mouseup / touchend / keyup), the parent issues a single
  `callCapability("set_brightness", { value: 0–255 })`.
- 500ms after `commit`, `userIsDragging` releases and the slider snaps back
  to following `value` from props.

## Error handling

- **Stream errors** → store transitions to `disconnected`, schedules
  reconnect with exponential backoff (1s, 2s, 4s, 8s, 16s, capped 30s,
  jittered ±10%). UI shows an inline pill in the entity section header
  while disconnected. Resume uses `lastCursor`.
- **CallCapability transport errors** → optimistic revert + inline error on the row (kept for ~3s, dismissable).
- **CallCapability domain errors** (`success: false, error_message`) → same as transport: revert + inline error showing `error_message`.
- **Watchdog timeout** (no heartbeat in 30s while `connected`) → force close and reconnect.

## Testing

- **Lab specimens** (deferred to a follow-up commit, listed in scope cuts) — every new lib component eventually gets a specimen in `app/src/views/lab/`.
- **Manual end-to-end** via Playwright on the running daemon + hue_main driver:
  1. Open Devices, open the rail for `hue_main`.
  2. Verify entity rows render and counts match `dist/switchyard entity list`.
  3. Toggle a light → confirm the bulb changes and the row reflects it.
  4. Drag brightness → confirm physical light tracks.
  5. Pull the daemon down → confirm "reconnecting" pill appears.
  6. Restart daemon → confirm stream resumes and state catches up.
- **No new Go-side tests** — the daemon's Subscribe path is already covered by `internal/registry`/`internal/entitystore` tests.

## Out-of-scope follow-ups (for future iterations)

- Toast / snackbar surface for non-row-scoped errors.
- Lab specimens for `SyEntityRow` and the `entity-controls/` widgets.
- Rooms drill-down (`/rooms/:id`) consuming `entityStore.byArea`.
- Home "Quick Controls" widget (pinned entities with inline toggles).
- Generic capability discovery for non-hue drivers — read available
  capabilities from `entity.capabilities` and render a generic widget per
  type rather than hardcoding the hue set.
