# Room detail dashboard implementation plan

> **For agentic workers:** Use superpowers:executing-plans to work through this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** A `/rooms/:id` detail page showing per-room entities (with controls), scenes, recent activity, and declared automations — backed by two small daemon extensions (Stories filter by entity-set, Pkl/proto support for automation→area linkage).

**Architecture:** UI shell ships first with entities + scenes (no backend changes). Iteration 2 extends `StoriesFilter` with `entity_ids[]` and adds the activity section. Iteration 3 adds Pkl `Automation.areas`, surfaces it through the engine + proto, and adds the automations section. Each iteration ends green and is independently usable.

**Tech Stack:** Go (daemon, eventstore, automation engine, ConnectRPC), Pkl (config schema), Vue 3 + TypeScript (UI), Playwright for E2E validation.

**Reference spec:** `docs/design/specs/2026-05-12-rooms-detail-dashboard-design.md`

**Verification strategy:** `go test ./...` for backend changes, `vue-tsc -b --noEmit` + Playwright for UI (no vitest in `app/` — same strategy used in 2026-05-11 entity-state work).

---

## File structure

### Iteration 1 (UI scaffold)

| Path | Action | Responsibility |
|---|---|---|
| `app/src/data/scenes.ts` | new | `listScenes()` + `applyScene(id)` Connect wrappers |
| `app/src/lib/components/scene/SyScene.vue` | new | Scene chip — name + busy state, emits `apply` |
| `app/src/lib/index.ts` | modify | Export `SyScene` |
| `app/src/views/RoomDetailView.vue` | new | The route component |
| `app/src/router/index.ts` | modify | Add `/rooms/:id` mapping |
| `app/src/router/crumbs.ts` | modify | Breadcrumb derivation for `/rooms/:id` |
| `app/src/views/RoomsView.vue` | modify | Tiles become clickable |

### Iteration 2 (Stories area scoping)

| Path | Action | Responsibility |
|---|---|---|
| `proto/switchyard/activity/v1/activity.proto` | modify | Add `repeated string entity_ids = 4` to `StoriesFilter` |
| `internal/activity/service.go` | modify | Filter logic — match story if any inner event's entity is in the set |
| `internal/activity/service_test.go` | modify | Coverage for entity_ids filter (singleton parity, set, union) |
| `app/src/data/activity.ts` | modify | `listStories({ entityIds })` accepts the new filter |
| `app/src/views/RoomDetailView.vue` | modify | Activity section becomes real |

### Iteration 3 (Automation area linkage)

| Path | Action | Responsibility |
|---|---|---|
| `internal/config/pkl/switchyard/automations.pkl` | modify | Add `areas: Listing<String> = new {}` to `Automation` |
| `proto/switchyard/config/v1/snapshot.proto` | modify | Add `repeated string areas = 14` to `AutomationConfig` |
| `internal/config/evaluator.go` | modify | Carry `areas` through `automationJSON` → `AutomationConfig` |
| `internal/automation/automation.go` | modify | Add `Areas []string` to runtime `Automation` |
| `internal/automation/compile.go` | modify | Copy `AutomationConfig.Areas` → runtime |
| `proto/switchyard/v1alpha1/automation.proto` | modify | Add `repeated string area_ids = 6` to `Automation`; `string area_id = 2` to `ListAutomationsRequest` |
| `internal/api/service_automation.go` | modify | Surface area_ids; honor area_id filter |
| `internal/api/service_automation_test.go` | modify | Coverage for area filter |
| `examples/automations/sunset-lights.pkl` (or new sample) | modify | Tag with `areas = ["bedroom"]` for E2E test |
| `app/src/data/automations.ts` | modify | `listAutomations({ areaId })` accepts the filter, exposes `areaIds` |
| `app/src/views/RoomDetailView.vue` | modify | Automations section becomes real |

---

# Iteration 1 — UI scaffold + entities + scenes

## Task 1.1: Scenes data client

**Files:**
- Create: `app/src/data/scenes.ts`

- [ ] **Step 1: Create the client module**

```ts
/**
 * SceneService client. Lists declared scenes and applies them.
 * Scenes are global (no area scoping in the proto); the room detail
 * view lists all scenes and lets the user pick one.
 */

import { rpcCall, type RpcOptions } from "./rpc";

const SCENE_SVC = "switchyard.v1alpha1.SceneService";

export interface Scene {
  id: string;
  displayName: string;
}

interface RawScene {
  id?: string;
  display_name?: string; displayName?: string;
}

function decode(r: RawScene): Scene {
  return {
    id:          r.id ?? "",
    displayName: r.displayName ?? r.display_name ?? "",
  };
}

export async function listScenes(opts: RpcOptions = {}): Promise<{ scenes: Scene[] }> {
  const res = await rpcCall<Record<string, never>, { scenes?: RawScene[] }>(
    `${SCENE_SVC}/List`, {}, opts,
  );
  return { scenes: (res.scenes ?? []).map(decode) };
}

export interface ApplySceneResult {
  correlationId: string;
}

export async function applyScene(id: string, opts: RpcOptions = {}): Promise<ApplySceneResult> {
  const res = await rpcCall<unknown, { correlationId?: string; correlation_id?: string }>(
    `${SCENE_SVC}/Apply`, { id }, opts,
  );
  return { correlationId: res.correlationId ?? res.correlation_id ?? "" };
}
```

- [ ] **Step 2: Typecheck**

Run: `cd app && npx vue-tsc --noEmit -p tsconfig.json 2>&1 | grep "scenes\.ts"`
Expected: empty output (no errors specific to scenes.ts).

- [ ] **Step 3: Playwright probe — confirm List works**

Open `http://localhost:5174` in Playwright. Evaluate:
```js
async () => {
  const m = await import("/src/data/scenes.ts");
  const r = await m.listScenes();
  return { count: r.scenes.length, sample: r.scenes[0] ?? null };
}
```
Expected: `{ count: <some number>, sample: { id: "...", displayName: "..." } }` or `{ count: 0, sample: null }` if no scenes are declared. No throw.

- [ ] **Step 4: Commit**

```bash
git add app/src/data/scenes.ts
git commit -m "feat(app): SceneService client (list + apply)"
```

---

## Task 1.2: SyScene chip component

**Files:**
- Create: `app/src/lib/components/scene/SyScene.vue`
- Modify: `app/src/lib/index.ts`

- [ ] **Step 1: Create the chip component**

```vue
<!--
  SyScene — chip that activates a scene on click. Pure presentational:
  emits `apply`, parent owns the busy/error state.
-->
<script setup lang="ts">
import { SyText, SyIcon } from "@/lib";

defineProps<{
  name: string;
  busy?: boolean;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "apply"): void;
}>();
</script>

<template>
  <button
    type="button"
    class="sy-scene"
    :class="{ 'sy-scene--busy': busy }"
    :disabled="disabled || busy"
    @click="emit('apply')"
  >
    <SyIcon name="sparkle" :size="14" />
    <SyText as="span" variant="body" weight="medium">{{ name }}</SyText>
  </button>
</template>

<style scoped>
.sy-scene {
  display: inline-flex; align-items: center; gap: var(--sy-space-2);
  padding: var(--sy-space-1) var(--sy-space-3);
  border: 1px solid var(--sy-color-line);
  border-radius: 999px;
  background: var(--sy-color-surface);
  color: var(--sy-color-fg);
  cursor: pointer;
  transition: background 120ms, border-color 120ms;
}
.sy-scene:hover:not(:disabled) {
  background: var(--sy-color-surface-hover);
  border-color: var(--sy-color-line-strong);
}
.sy-scene:disabled { cursor: default; opacity: 0.6; }
.sy-scene--busy { opacity: 0.7; }
</style>
```

- [ ] **Step 2: Add to library exports**

Edit `app/src/lib/index.ts` — append after the last `export { default as Sy... }` line:
```ts
export { default as SyScene } from "./components/scene/SyScene.vue";
```

- [ ] **Step 3: Typecheck**

Run: `cd app && npx vue-tsc --noEmit -p tsconfig.json 2>&1 | grep -E "scene/Sy|lib/index"`
Expected: empty output.

- [ ] **Step 4: Commit**

```bash
git add app/src/lib/components/scene/ app/src/lib/index.ts
git commit -m "feat(app): SyScene chip component"
```

---

## Task 1.3: Router + crumbs for /rooms/:id

**Files:**
- Modify: `app/src/router/index.ts`
- Modify: `app/src/router/crumbs.ts`
- Modify: `app/src/views/RoomsView.vue`

- [ ] **Step 1: Add the route**

Open `app/src/router/index.ts`. Find the line `{ path: "rooms",       name: "rooms",       component: RoomsView },` and add directly after it:
```ts
      { path: "rooms/:id",   name: "room-detail", component: () => import("@/views/RoomDetailView.vue") },
```
(Lazy import keeps the initial bundle smaller and breaks the would-be circular import between this route and components.)

- [ ] **Step 2: Add breadcrumb derivation**

Open `app/src/router/crumbs.ts`. Find the function `crumbsFor` (it's a switch on `route.name`). Add a new case before the default:
```ts
    case "room-detail": {
      const id = String(route.params.id ?? "");
      return [
        { label: "Rooms", to: "/rooms" },
        { label: id || "Room" },
      ];
    }
```

(The label uses the raw id as a fallback. RoomDetailView itself will surface the friendly area name in the page header; the breadcrumb is intentionally minimal so it doesn't need an async data fetch.)

- [ ] **Step 3: Make RoomsView tiles clickable**

Open `app/src/views/RoomsView.vue`. Find the `<SyRoomTile` template usage. Add an `href` (or `to`) prop. Check existing SyRoomTile API first:
```bash
grep -nE "defineProps|href\b|to\b" app/src/lib/components/room-tile/SyRoomTile.vue | head -10
```
If the component takes `href`, set `:href="\`/rooms/${a.id}\`"`. If it takes `to` (Vue Router), set `:to="\`/rooms/${a.id}\`"`. If neither, wrap the tile in a `RouterLink`:
```vue
<RouterLink
  v-for="a in areas"
  :key="a.id"
  :to="`/rooms/${a.id}`"
  custom
  v-slot="{ navigate }"
>
  <SyRoomTile
    :name="a.displayName"
    :meta="`${entityCount(a.id)} ${entityCount(a.id) === 1 ? 'entity' : 'entities'}`"
    @click="navigate"
  >
    ...existing slot content...
  </SyRoomTile>
</RouterLink>
```
Add `import { RouterLink } from "vue-router";` to the script block if not already imported.

- [ ] **Step 4: Typecheck — expect this to fail until Task 1.4 creates RoomDetailView**

Run: `cd app && npx vue-tsc --noEmit -p tsconfig.json 2>&1 | grep -E "router/index"`
Expected: an error about `@/views/RoomDetailView.vue` not existing. We'll resolve this in Task 1.4.

- [ ] **Step 5: (No commit yet — wait for Task 1.4 to create the view)**

---

## Task 1.4: RoomDetailView (entities + scenes)

**Files:**
- Create: `app/src/views/RoomDetailView.vue`

- [ ] **Step 1: Create the view**

```vue
<!--
  RoomDetailView — the per-room dashboard at /rooms/:id.

  Sections (in order):
    1. Header: room name, entity-type breakdown, on/off summary
    2. Scenes: chip row, click to apply
    3. Entities: SyEntityRow grouped by type
    4. Activity: deferred to Iteration 2 (placeholder empty state)
    5. Automations: deferred to Iteration 3 (placeholder empty state)

  Reads entity state live via entityStore.byArea. Scenes are listed
  globally — there's no per-area scene scoping in the proto today.
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  SyText, SySurface, SyButton, SyIcon, SyEmptyState, SyBadge,
  SyEntityRow, SyScene,
} from "@/lib";
import { listAreas, type Area } from "@/data/areas";
import { listScenes, applyScene, type Scene } from "@/data/scenes";
import { entityStore } from "@/stores/entity-store";
import type { Entity } from "@/data/entities";

const route = useRoute();
const router = useRouter();

const areaId = computed<string>(() => String(route.params.id ?? ""));

/* ---- Area name ----------------------------------------------------- */
const areas = ref<Area[]>([]);
const areasLoaded = ref<boolean>(false);
const area = computed<Area | null>(() => areas.value.find((a) => a.id === areaId.value) ?? null);
const areaName = computed<string>(() => area.value?.displayName || areaId.value);

/* ---- Entities (live from store) ------------------------------------ */
const entities = computed<Entity[]>(() => entityStore.byArea(areaId.value).value);

const entitiesByKind = computed<{ light: Entity[]; switch: Entity[]; sensor: Entity[]; other: Entity[] }>(() => {
  const out = { light: [] as Entity[], switch: [] as Entity[], sensor: [] as Entity[], other: [] as Entity[] };
  for (const e of entities.value) {
    if (e.state?.light)         out.light.push(e);
    else if (e.state?.switchDevice) out.switch.push(e);
    else if (e.state?.numericSensor || e.state?.binarySensor) out.sensor.push(e);
    else                            out.other.push(e);
  }
  return out;
});

const onCount = computed<number>(() => entities.value.filter((e) => e.state?.light?.on || e.state?.switchDevice?.on).length);
const offCount = computed<number>(() => entities.value.length - onCount.value);

/* ---- Scenes -------------------------------------------------------- */
const scenes = ref<Scene[]>([]);
const scenesLoading = ref<boolean>(true);
const scenesError = ref<string>("");
const scenesBusy = ref<Set<string>>(new Set());
const sceneError = ref<string>("");

let abort: AbortController | null = null;

async function loadAreas(): Promise<void> {
  try {
    const r = await listAreas();
    areas.value = r.areas;
  } catch { /* surface as 'unknown room' below */ }
  finally { areasLoaded.value = true; }
}

async function loadScenes(): Promise<void> {
  scenesLoading.value = true;
  scenesError.value = "";
  try {
    const r = await listScenes();
    scenes.value = r.scenes;
  } catch (err) {
    scenesError.value = err instanceof Error ? err.message : String(err);
  } finally { scenesLoading.value = false; }
}

async function onApplyScene(s: Scene): Promise<void> {
  scenesBusy.value = new Set(scenesBusy.value).add(s.id);
  sceneError.value = "";
  try {
    await applyScene(s.id);
  } catch (err) {
    sceneError.value = err instanceof Error ? err.message : String(err);
  } finally {
    const next = new Set(scenesBusy.value);
    next.delete(s.id);
    scenesBusy.value = next;
  }
}

onMounted(() => {
  void loadAreas();
  void loadScenes();
});
onBeforeUnmount(() => { abort?.abort(); });

/* ---- Empty / unknown room ----------------------------------------- */
const isUnknownRoom = computed<boolean>(() =>
  areasLoaded.value && entityStore.hydrated.value && !area.value && entities.value.length === 0);
</script>

<template>
  <div class="page">
    <!-- Unknown room: show a single big empty state instead of broken sections. -->
    <SySurface v-if="isUnknownRoom" padding="none">
      <SyEmptyState
        title="This room doesn't exist"
        :description="`No area with id '${areaId}' is registered.`"
      >
        <template #icon><SyIcon name="rooms" :size="28" /></template>
        <template #actions>
          <SyButton intent="primary" @click="router.push('/rooms')">Back to Rooms</SyButton>
        </template>
      </SyEmptyState>
    </SySurface>

    <template v-else>
      <header class="page__head">
        <SyText as="h1" variant="display">{{ areaName }}</SyText>
        <SyText variant="body" tone="subtle">
          {{ entities.length }}
          {{ entities.length === 1 ? "entity" : "entities" }}
          <template v-if="entities.length > 0">
            · {{ onCount }} on / {{ offCount }} off
          </template>
        </SyText>
      </header>

      <!-- Scenes -->
      <section v-if="scenesLoading || scenes.length > 0 || scenesError" class="page__section">
        <SyText variant="title" weight="semibold">Scenes</SyText>
        <SyText v-if="scenesLoading" variant="caption" tone="subtle">Loading…</SyText>
        <SyText v-else-if="scenesError" variant="caption" tone="bad">{{ scenesError }}</SyText>
        <div v-else class="page__sceneRow">
          <SyScene
            v-for="s in scenes"
            :key="s.id"
            :name="s.displayName || s.id"
            :busy="scenesBusy.has(s.id)"
            @apply="onApplyScene(s)"
          />
        </div>
        <SyText v-if="sceneError" variant="caption" tone="bad">{{ sceneError }}</SyText>
      </section>

      <!-- Entities -->
      <section class="page__section">
        <SyText variant="title" weight="semibold">Entities</SyText>
        <SyEmptyState
          v-if="entities.length === 0"
          size="compact"
          title="No entities in this room"
          description="Assign entities to this area in your Pkl config."
        />
        <template v-else>
          <div v-if="entitiesByKind.light.length > 0" class="page__group">
            <SyText variant="label" tone="subtle">Lights</SyText>
            <SySurface padding="none" class="page__list">
              <SyEntityRow v-for="e in entitiesByKind.light" :key="e.id" :entity="e" />
            </SySurface>
          </div>
          <div v-if="entitiesByKind.switch.length > 0" class="page__group">
            <SyText variant="label" tone="subtle">Switches</SyText>
            <SySurface padding="none" class="page__list">
              <SyEntityRow v-for="e in entitiesByKind.switch" :key="e.id" :entity="e" />
            </SySurface>
          </div>
          <div v-if="entitiesByKind.sensor.length > 0" class="page__group">
            <SyText variant="label" tone="subtle">Sensors</SyText>
            <SySurface padding="none" class="page__list">
              <SyEntityRow v-for="e in entitiesByKind.sensor" :key="e.id" :entity="e" />
            </SySurface>
          </div>
          <div v-if="entitiesByKind.other.length > 0" class="page__group">
            <SyText variant="label" tone="subtle">Other</SyText>
            <SySurface padding="none" class="page__list">
              <SyEntityRow v-for="e in entitiesByKind.other" :key="e.id" :entity="e" />
            </SySurface>
          </div>
        </template>
      </section>

      <!-- Activity (Iteration 2) -->
      <section class="page__section">
        <div class="page__sectionHead">
          <SyText variant="title" weight="semibold">Recent activity</SyText>
          <SyButton intent="ghost" size="sm" @click="router.push('/activity')">
            View all
            <SyIcon name="chevron-right" :size="12" />
          </SyButton>
        </div>
        <SySurface padding="none">
          <SyEmptyState
            size="compact"
            title="Coming soon"
            description="Per-room activity scoping ships in iteration 2."
          />
        </SySurface>
      </section>

      <!-- Automations (Iteration 3) -->
      <section class="page__section">
        <SyText variant="title" weight="semibold">Automations</SyText>
        <SySurface padding="none">
          <SyEmptyState
            size="compact"
            title="Coming soon"
            description="Per-room automation scoping ships in iteration 3."
          />
        </SySurface>
      </section>
    </template>
  </div>
</template>

<style scoped>
.page {
  padding: var(--sy-space-5) var(--sy-space-6);
  display: flex; flex-direction: column;
  gap: var(--sy-space-5);
  max-width: 1080px;
}
.page__head { display: flex; flex-direction: column; gap: var(--sy-space-1); }
.page__section { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.page__sectionHead { display: flex; align-items: center; justify-content: space-between; }
.page__sceneRow { display: flex; flex-wrap: wrap; gap: var(--sy-space-2); }
.page__group { display: flex; flex-direction: column; gap: var(--sy-space-1); }
.page__list :deep(.sy-er + .sy-er) { border-top: 1px solid var(--sy-color-line-soft); }
</style>
```

- [ ] **Step 2: Typecheck**

Run: `cd app && npx vue-tsc --noEmit -p tsconfig.json 2>&1 | grep -E "RoomDetailView|router/index"`
Expected: empty output (router/index error from Task 1.3 resolves now that the file exists).

- [ ] **Step 3: Playwright validation**

Open `http://localhost:5174/rooms` in Playwright. Snapshot. Click any room tile. Verify URL becomes `/rooms/<area-id>` and the page renders header + scenes (or "Loading…") + entities grouped by type. Take screenshot `room-detail-iter1.png`. Click a scene chip — verify it goes busy briefly and no inline error appears (SceneService.Apply may 500 if no scene-mapped entities exist, in which case the inline error is the expected outcome — that's still validation that the round-trip works).

If the daemon's SceneService is unimplemented, the chips just won't render (`scenes.length === 0`). The Activity + Automations "Coming soon" cards should appear regardless.

- [ ] **Step 4: Commit Tasks 1.3 + 1.4 together**

```bash
git add app/src/router/index.ts app/src/router/crumbs.ts app/src/views/RoomsView.vue app/src/views/RoomDetailView.vue
git commit -m "feat(app): RoomDetailView + clickable rooms tiles + /rooms/:id route"
```

---

## Task 1.5: Iteration 1 cross-page sanity sweep

- [ ] **Step 1: Tour all pages via Playwright, confirm no regressions**

Visit /, /rooms (click each tile if possible), /rooms/<id> for one or two areas, /activity, /automations, /devices, /settings. Confirm:
- No new console errors (favicon 404 is expected baseline).
- Entity store still hydrated and connected (eval `entityStore` from a probe).
- Breadcrumb on /rooms/:id reads "Rooms / <id>".

- [ ] **Step 2: Iteration 1 complete — checkpoint**

No commit needed unless sanity sweep surfaces a fix. Iteration 1 ships the page with entities + scenes; Activity and Automations sections render their "Coming soon" empty states.

---

# Iteration 2 — Stories area scoping

## Task 2.1: Add `entity_ids` to StoriesFilter proto

**Files:**
- Modify: `proto/switchyard/activity/v1/activity.proto`

- [ ] **Step 1: Add the field**

Open the proto. Find the `StoriesFilter` message. Add `entity_ids` after the existing `entity_id`:
```diff
 message StoriesFilter {
   // 1-9: event filters
   string   kind        = 1;
   string   source      = 2;
   string   entity_id   = 3;
+  // entity_ids matches a story if ANY inner event's entity is in the
+  // set. When both entity_id and entity_ids are set, treat as union.
+  repeated string entity_ids = 4;

   // 10-19: interestingness filters
```

- [ ] **Step 2: Regenerate Connect / Go bindings**

Run: `buf generate` (from repo root)
Expected: no errors. New field appears in `gen/switchyard/activity/v1/activity.pb.go` as `EntityIds []string` on `StoriesFilter`.

Verify: `grep -n "EntityIds" gen/switchyard/activity/v1/activity.pb.go | head -3`
Expected: at least one match showing `EntityIds []string`.

- [ ] **Step 3: Confirm build still passes**

Run: `go build ./...`
Expected: clean build, no errors.

- [ ] **Step 4: Commit**

```bash
git add proto/switchyard/activity/v1/activity.proto gen/switchyard/activity/v1/
git commit -m "proto(activity): StoriesFilter.entity_ids — match any inner event entity in set"
```

---

## Task 2.2: Daemon filter logic for `entity_ids`

**Files:**
- Modify: `internal/activity/service.go`
- Modify: `internal/activity/service_test.go`

- [ ] **Step 1: Locate the existing filter site**

Run: `grep -nE "EntityId\b|entity_id\b|StoriesFilter" internal/activity/service.go | head -20`
Expected: one or more hits showing where `filter.GetEntityId()` is consulted in the Stories handler.

- [ ] **Step 2: Write the failing tests first**

Open `internal/activity/service_test.go`. Add at the end (or near other Stories tests):

```go
func TestStories_EntityIdsFilter_MatchesSet(t *testing.T) {
	svc, store := newActivityServiceForTest(t)

	// Seed three state_changed events, each touching a different entity.
	mustAppend(t, store, testutil.StateChanged("light.kitchen", 1))
	mustAppend(t, store, testutil.StateChanged("light.bedroom", 2))
	mustAppend(t, store, testutil.StateChanged("light.office",  3))

	resp, err := svc.Stories(context.Background(), connect.NewRequest(&activityv1.StoriesRequest{
		Filter: &activityv1.StoriesFilter{
			EntityIds: []string{"light.kitchen", "light.bedroom"},
		},
	}))
	if err != nil {
		t.Fatalf("Stories: %v", err)
	}
	got := storyEntities(resp.Msg.Stories)
	want := map[string]bool{"light.kitchen": true, "light.bedroom": true}
	if !equalEntitySets(got, want) {
		t.Fatalf("entity_ids filter returned %v, want %v", got, want)
	}
}

func TestStories_EntityIdsFilter_UnionWithSingular(t *testing.T) {
	svc, store := newActivityServiceForTest(t)
	mustAppend(t, store, testutil.StateChanged("light.kitchen", 1))
	mustAppend(t, store, testutil.StateChanged("light.bedroom", 2))
	mustAppend(t, store, testutil.StateChanged("light.office",  3))

	resp, err := svc.Stories(context.Background(), connect.NewRequest(&activityv1.StoriesRequest{
		Filter: &activityv1.StoriesFilter{
			EntityId:  "light.kitchen",
			EntityIds: []string{"light.bedroom"},
		},
	}))
	if err != nil {
		t.Fatalf("Stories: %v", err)
	}
	got := storyEntities(resp.Msg.Stories)
	want := map[string]bool{"light.kitchen": true, "light.bedroom": true}
	if !equalEntitySets(got, want) {
		t.Fatalf("union filter returned %v, want %v", got, want)
	}
}

// Helpers — add only if newActivityServiceForTest / storyEntities don't
// already exist in this test file.
func storyEntities(stories []*activityv1.Story) map[string]bool {
	out := make(map[string]bool)
	for _, s := range stories {
		// A Story's primary entity is the entity of its first inner event.
		// Tests use single-event stories so primary == only.
		if len(s.InnerEventIds) == 0 {
			continue
		}
		out[lookupEventEntity(s.InnerEventIds[0])] = true
	}
	return out
}

func equalEntitySets(a, b map[string]bool) bool {
	if len(a) != len(b) { return false }
	for k := range a { if !b[k] { return false } }
	return true
}
```

If `newActivityServiceForTest`, `mustAppend`, `lookupEventEntity` don't already exist in this file, locate equivalents elsewhere in `internal/activity/*_test.go` and reuse them. If nothing similar exists, this is a sign the test file is too thin — pause and ask before inventing fixtures.

- [ ] **Step 3: Run tests; expect failure**

Run: `go test -run "TestStories_EntityIdsFilter" ./internal/activity/... -v`
Expected: failure with "entity_ids filter returned …, want …" or similar — the new field is in the proto but isn't being honored by the handler yet.

- [ ] **Step 4: Implement filter logic**

In `internal/activity/service.go`, locate the Stories handler. Find where `filter.GetEntityId()` (singular) is consulted. Extend with set semantics. The exact integration point depends on the handler shape; here's the pattern:

```go
// Before this block, the handler already calls coalescer.QueryStories or
// similar. Wrap the existing entity-id filter with a set-aware predicate.

allowed := func(entityID string) bool {
    singular := filter.GetEntityId()
    set := filter.GetEntityIds()
    if singular == "" && len(set) == 0 {
        return true // no entity filter — let everything through
    }
    if singular != "" && entityID == singular {
        return true
    }
    for _, id := range set {
        if id == entityID { return true }
    }
    return false
}
```

Apply `allowed(...)` to each candidate story's primary entity (or, if the existing filter operates on raw events, to each inner event before coalescing). Match the existing code's level of granularity — don't introduce a new filter site if one already exists; extend the one that does.

- [ ] **Step 5: Run tests; expect pass**

Run: `go test -run "TestStories_EntityIdsFilter" ./internal/activity/... -v`
Expected: PASS for both new tests, plus all existing Stories tests still pass.

- [ ] **Step 6: Run full activity package tests**

Run: `go test ./internal/activity/...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/activity/service.go internal/activity/service_test.go
git commit -m "feat(activity): StoriesFilter.entity_ids — match any inner event entity in set"
```

---

## Task 2.3: TS client — `listStories` accepts `entityIds`

**Files:**
- Modify: `app/src/data/activity.ts`

- [ ] **Step 1: Locate the existing listStories shape**

Run: `grep -nE "listStories|StoriesFilter|entity_id" app/src/data/activity.ts | head -10`
Note the existing function signature.

- [ ] **Step 2: Add `entityIds` to the filter type**

Find the local TypeScript filter type (probably `interface StoriesFilter` or inline in `listStories`). Add the optional field:
```ts
entityIds?: string[];
```

In the request body construction, include `entity_ids` (or `entityIds`, depending on whether the existing client uses snake or camel — match the prevailing style; the daemon decodes both):
```ts
{ filter: { ..., entityIds: opts.entityIds ?? undefined } }
```

- [ ] **Step 3: Typecheck**

Run: `cd app && npx vue-tsc --noEmit -p tsconfig.json 2>&1 | grep "activity\.ts"`
Expected: empty.

- [ ] **Step 4: Playwright probe**

Open `http://localhost:5174` in Playwright. Evaluate:
```js
async () => {
  const m = await import("/src/data/activity.ts");
  const r = await m.listStories({ entityIds: ["light.hue_03593af4"] });
  return { count: r.stories.length, sampleId: r.stories[0]?.id };
}
```
Expected: a count (possibly 0 if no recent events for that entity) and no throw.

- [ ] **Step 5: Commit**

```bash
git add app/src/data/activity.ts
git commit -m "feat(app): listStories accepts entityIds filter"
```

---

## Task 2.4: Wire Activity section in RoomDetailView

**Files:**
- Modify: `app/src/views/RoomDetailView.vue`

- [ ] **Step 1: Replace the placeholder section with real data**

In the script block, add imports + state alongside existing scene/entity logic:
```ts
import { listStories, type Story } from "@/data/activity";
import { presentEvent } from "@/data/event-display"; // if presenting events directly

const stories = ref<Story[]>([]);
const storiesLoading = ref<boolean>(true);
const storiesError = ref<string>("");

async function loadStories(): Promise<void> {
  storiesLoading.value = true;
  storiesError.value = "";
  try {
    const ids = entities.value.map((e) => e.id);
    if (ids.length === 0) {
      stories.value = [];
      return;
    }
    const r = await listStories({ entityIds: ids });
    stories.value = r.stories.slice(0, 5);
  } catch (err) {
    storiesError.value = err instanceof Error ? err.message : String(err);
  } finally { storiesLoading.value = false; }
}

// Reload whenever the entity set for this area changes (e.g., on first
// hydration). watch is already imported from vue elsewhere in this file
// — if not, add it to the imports.
import { watch } from "vue";
watch(() => entities.value.map((e) => e.id).join(","), () => { void loadStories(); });

onMounted(() => {
  // Existing: void loadAreas(); void loadScenes();
  void loadStories();
});
```

- [ ] **Step 2: Replace the "Coming soon" Activity section template**

```vue
<!-- Activity -->
<section class="page__section">
  <div class="page__sectionHead">
    <SyText variant="title" weight="semibold">Recent activity</SyText>
    <SyButton intent="ghost" size="sm" @click="router.push('/activity')">
      View all
      <SyIcon name="chevron-right" :size="12" />
    </SyButton>
  </div>

  <SySurface v-if="storiesLoading" padding="none">
    <SyEmptyState loading title="Loading recent activity…" />
  </SySurface>

  <SySurface v-else-if="storiesError" padding="none">
    <SyEmptyState
      intent="bad"
      title="Couldn't load activity"
      :description="storiesError"
    >
      <template #icon><SyIcon name="close" :size="28" /></template>
      <template #actions>
        <SyButton intent="secondary" @click="loadStories">Retry</SyButton>
      </template>
    </SyEmptyState>
  </SySurface>

  <SySurface v-else-if="stories.length === 0" padding="none">
    <SyEmptyState
      size="compact"
      title="Quiet over here"
      description="No recent activity for this room."
    />
  </SySurface>

  <SySurface v-else padding="none" class="page__list">
    <!-- The exact SyEventRow/SyStoryRow shape depends on what the
         Activity page already uses. If presentEvent expects an
         EventRecord but stories carry inner_event_ids, render a story
         row instead — match the pattern in app/src/views/ActivityView.vue
         to avoid divergence. -->
    <pre style="padding: var(--sy-space-3); margin: 0; font-size: 12px;">{{ stories }}</pre>
  </SySurface>
</section>
```

> **Note for executor:** The placeholder `<pre>{{ stories }}</pre>` block is a deliberate stop-and-look. Before replacing it with `SyStoryRow` or `SyEventRow`, open `app/src/views/ActivityView.vue` and pattern-match the rendering it uses. Reuse the same component + `presentEvent` (or its story-equivalent) so the room-detail Activity section reads identically to the global Activity page. Do not invent a new presentation.

- [ ] **Step 3: Match Activity page presentation**

Open `app/src/views/ActivityView.vue`. Find the loop that renders stories. Note the component name (likely `SyStoryRow`) and any `presentStory(...)` helper. Replace the `<pre>` placeholder with the matching markup. Import what you need.

- [ ] **Step 4: Typecheck**

Run: `cd app && npx vue-tsc --noEmit -p tsconfig.json 2>&1 | grep "RoomDetailView"`
Expected: empty.

- [ ] **Step 5: Playwright validation**

Open `http://localhost:5174/rooms/<an-area-with-entities>`. Verify the Activity section now shows real stories filtered to entities in the room. Trigger a state change (e.g., toggle one of the room's lights) — confirm a new story appears within a few seconds.

Take screenshot `room-detail-iter2.png`.

- [ ] **Step 6: Commit**

```bash
git add app/src/views/RoomDetailView.vue
git commit -m "feat(app): RoomDetailView Activity section — area-scoped stories"
```

---

## Task 2.5: Iteration 2 sanity sweep

- [ ] **Step 1: Quick cross-page tour**

Visit /, /rooms, /rooms/<id>, /activity, /devices. Confirm no regressions, no console errors. Confirm the global Activity page still works (we extended `listStories` but back-compat should hold).

---

# Iteration 3 — Automation area linkage

## Task 3.1: Pkl Automation.areas field

**Files:**
- Modify: `internal/config/pkl/switchyard/automations.pkl`

- [ ] **Step 1: Add the field to the Automation class**

Open the Pkl file. Find the `class Automation { ... }` block. Add the field after `enabled`:
```diff
 class Automation {
   id:         String(!isEmpty)
   triggers:   Listing<Trigger>
   conditions: Listing<Condition> = new {}
   actions:    Listing<Action>
   mode:       String(this == "single" || this == "queued" || this == "restart" || this == "parallel") = "single"
   maxQueued:  Int = 10
   enabled:    Boolean = true
+  // areas declares the rooms this automation operates on. Surfaces
+  // through AutomationConfig.areas → Automation.area_ids on the wire,
+  // which the UI uses to scope the per-room automations section.
+  areas:      Listing<String> = new {}
   onFailure:  OnFailureStrategy = new IgnoreStrategy {}
 }
```

- [ ] **Step 2: Verify the daemon still parses existing configs**

Run: `go test ./internal/config/...`
Expected: PASS — unchanged Pkl files should still evaluate (the new `areas` field has a default of `new {}` so existing configs without it remain valid).

- [ ] **Step 3: Commit**

```bash
git add internal/config/pkl/switchyard/automations.pkl
git commit -m "pkl: Automation.areas — declare rooms an automation operates on"
```

---

## Task 3.2: AutomationConfig proto + evaluator carry-through

**Files:**
- Modify: `proto/switchyard/config/v1/snapshot.proto`
- Modify: `internal/config/evaluator.go`

- [ ] **Step 1: Add the field to the AutomationConfig proto**

Open `proto/switchyard/config/v1/snapshot.proto`. Find `message AutomationConfig`. Add `areas` at field number 14 (next free in the 10-19 nested-config range):
```diff
 message AutomationConfig {
   // 1-9: identity & flags
   string id         = 1;
   bool   enabled    = 2;
   Mode   mode       = 3;
   int32  max_queued = 4;

   // 10-19: nested config
   repeated TriggerConfig   triggers   = 10;
   repeated ConditionConfig conditions = 11;
   repeated ActionConfig    actions    = 12;
   OnFailureConfig          on_failure = 13;
+  repeated string          areas      = 14;
 }
```

- [ ] **Step 2: Regenerate Go bindings**

Run: `buf generate`
Verify: `grep -n "Areas" gen/switchyard/config/v1/snapshot.pb.go | head -3`
Expected: shows `Areas []string` on `AutomationConfig`.

- [ ] **Step 3: Carry `areas` through the Pkl evaluator**

Open `internal/config/evaluator.go`. Find `type automationJSON struct { ... }`. Add the field:
```diff
 type automationJSON struct {
   ID         string            `json:"id"`
   Enabled    bool              `json:"enabled"`
   Mode       string            `json:"mode"`
   MaxQueued  int32             `json:"maxQueued"`
   Triggers   []json.RawMessage `json:"triggers"`
   Conditions []json.RawMessage `json:"conditions"`
   Actions    []json.RawMessage `json:"actions"`
+  Areas      []string          `json:"areas"`
 }
```

Then find the loop that builds `acfg := &configpb.AutomationConfig{...}`. Add `Areas` in the struct literal:
```diff
 acfg := &configpb.AutomationConfig{
     Id:        strings.TrimSpace(a.ID),
     Enabled:   a.Enabled,
     Mode:      parseAutomationMode(a.Mode),
     MaxQueued: a.MaxQueued,
+    Areas:     a.Areas,
 }
```

- [ ] **Step 4: Build + run config tests**

Run: `go build ./... && go test ./internal/config/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add proto/switchyard/config/v1/snapshot.proto gen/switchyard/config/v1/ internal/config/evaluator.go
git commit -m "proto+config: AutomationConfig.areas — carry Pkl areas through evaluator"
```

---

## Task 3.3: Runtime Automation + compile.go

**Files:**
- Modify: `internal/automation/automation.go`
- Modify: `internal/automation/compile.go`

- [ ] **Step 1: Add `Areas []string` to runtime Automation**

Open `internal/automation/automation.go`. Find `type Automation struct { ... }`. Add:
```diff
 type Automation struct {
   ID         string
   Triggers   []trigger.Matcher
   Conditions []condition.Evaluator
   Actions    []action.Executor
   ActionCtrl []action.ChildCtrl
   Mode       Mode
   MaxQueued  int
   Enabled    bool
+  Areas      []string

   Source *configpb.AutomationConfig
 }
```

- [ ] **Step 2: Copy AutomationConfig.Areas into the runtime struct**

Open `internal/automation/compile.go`. Find `compileOneWithMetrics` (or `compileOne`) where the runtime `Automation` is constructed. Add the `Areas` assignment in the literal/builder. The exact position depends on the existing code shape — locate the `&Automation{...}` literal and add:
```diff
 a := &Automation{
     ID:         ac.GetId(),
     // …existing fields…
+    Areas:      append([]string(nil), ac.GetAreas()...),
     Source:     ac,
 }
```

(`append([]string(nil), …)` defensively copies so callers can't mutate the proto's slice via the runtime struct.)

- [ ] **Step 3: Run automation tests**

Run: `go test ./internal/automation/...`
Expected: PASS — no test should break since we only added a field.

- [ ] **Step 4: Commit**

```bash
git add internal/automation/automation.go internal/automation/compile.go
git commit -m "automation: surface Areas on runtime Automation"
```

---

## Task 3.4: API Automation proto + ListAutomationsRequest filter

**Files:**
- Modify: `proto/switchyard/v1alpha1/automation.proto`

- [ ] **Step 1: Add `area_ids` and `area_id` filter**

Open the proto. Edit:
```diff
 message Automation {
   string id           = 1;
   string display_name = 2;
   string mode         = 3;
   bool   enabled      = 4;
   uint32 in_flight    = 5;
+  // area_ids surfaces the Pkl areas declaration; UI uses this for
+  // the per-room dashboard's automations section.
+  repeated string area_ids = 6;
 }

 message ListAutomationsRequest {
   PageRequest page = 1;
+  // area_id filters to automations whose areas include this id.
+  string area_id   = 2;
 }
```

- [ ] **Step 2: Regenerate**

Run: `buf generate`
Verify: `grep -nE "AreaIds|AreaId" gen/switchyard/v1alpha1/automation.pb.go | head -3`
Expected: matches showing the new fields.

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add proto/switchyard/v1alpha1/automation.proto gen/switchyard/v1alpha1/
git commit -m "proto(automation): area_ids on Automation, area_id filter on ListAutomationsRequest"
```

---

## Task 3.5: AutomationService.List honors area filter + populates area_ids

**Files:**
- Modify: `internal/api/service_automation.go`
- Modify: `internal/api/service_automation_test.go`

- [ ] **Step 1: Locate the List handler**

Run: `grep -n "func .*Service.*List\|automations\s*\[\]\*v1\.Automation\b" internal/api/service_automation.go | head -5`
Note the loop that converts internal automations to proto.

- [ ] **Step 2: Write the failing tests**

Open `internal/api/service_automation_test.go`. Add:

```go
func TestAutomationService_List_PopulatesAreaIds(t *testing.T) {
	svc := newAutomationServiceForTest(t, []*automation.Automation{
		{ID: "morning",  Areas: []string{"bedroom", "kitchen"}},
		{ID: "evening",  Areas: nil},
	})
	resp, err := svc.List(context.Background(), connect.NewRequest(&v1.ListAutomationsRequest{}))
	if err != nil { t.Fatalf("List: %v", err) }
	got := map[string][]string{}
	for _, a := range resp.Msg.Automations {
		got[a.Id] = a.AreaIds
	}
	if !reflect.DeepEqual(got["morning"], []string{"bedroom", "kitchen"}) {
		t.Fatalf("morning AreaIds = %v", got["morning"])
	}
	if len(got["evening"]) != 0 {
		t.Fatalf("evening AreaIds = %v, want empty", got["evening"])
	}
}

func TestAutomationService_List_FiltersByAreaId(t *testing.T) {
	svc := newAutomationServiceForTest(t, []*automation.Automation{
		{ID: "morning", Areas: []string{"bedroom"}},
		{ID: "evening", Areas: []string{"kitchen"}},
		{ID: "daily",   Areas: []string{"bedroom", "kitchen"}},
		{ID: "global",  Areas: nil},
	})
	resp, err := svc.List(context.Background(), connect.NewRequest(&v1.ListAutomationsRequest{
		AreaId: "bedroom",
	}))
	if err != nil { t.Fatalf("List: %v", err) }
	ids := []string{}
	for _, a := range resp.Msg.Automations { ids = append(ids, a.Id) }
	sort.Strings(ids)
	want := []string{"daily", "morning"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("filter result = %v, want %v", ids, want)
	}
}
```

If `newAutomationServiceForTest` doesn't exist, locate the existing test fixture pattern in this file and reuse it. If no clean fixture exists, build one inline using the smallest test double that satisfies `AutomationService`'s constructor — pause and look at how other handlers in `internal/api/*_test.go` set up service-level tests (`service_entity_test.go` is a good reference).

- [ ] **Step 3: Run tests; expect failure**

Run: `go test -run "TestAutomationService_List" ./internal/api/... -v`
Expected: failures because the proto fields aren't yet honored by the handler.

- [ ] **Step 4: Implement in service_automation.go**

Find the loop that builds the response. Two changes:
1. Set `AreaIds: a.Areas` on each `&v1.Automation{...}` (or after construction).
2. Skip entries where `req.Msg.GetAreaId()` is set and `a.Areas` doesn't contain it.

Pattern:
```go
filterArea := req.Msg.GetAreaId()
out := make([]*v1.Automation, 0, len(automations))
for _, a := range automations {
    if filterArea != "" && !containsString(a.Areas, filterArea) {
        continue
    }
    out = append(out, &v1.Automation{
        Id:          a.ID,
        DisplayName: a.DisplayName(),
        Mode:        a.Mode.String(),
        Enabled:     a.Enabled,
        InFlight:    uint32(a.InFlight()),
        AreaIds:     append([]string(nil), a.Areas...),
    })
}
```
Add a small `containsString` helper in the same file if there isn't one already.

- [ ] **Step 5: Run tests; expect pass**

Run: `go test -run "TestAutomationService_List" ./internal/api/... -v`
Expected: PASS for both new tests.

- [ ] **Step 6: Run full api tests**

Run: `go test ./internal/api/...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/service_automation.go internal/api/service_automation_test.go
git commit -m "feat(automation): List populates area_ids; honors area_id filter"
```

---

## Task 3.6: TS client — `listAutomations` accepts `areaId`, exposes `areaIds`

**Files:**
- Modify: `app/src/data/automations.ts`

- [ ] **Step 1: Extend the client**

Open the file. Find the `Automation` interface and `listAutomations` function.

Add `areaIds: string[]` to the interface; map from `area_ids` (snake) or `areaIds` (camel) in decode:
```ts
interface RawAutomation {
  // existing fields…
  area_ids?: string[]; areaIds?: string[];
}

function decode(r: RawAutomation): Automation {
  return {
    // existing field copies…
    areaIds: r.areaIds ?? r.area_ids ?? [],
  };
}
```

Add an optional `areaId` to `listAutomations`:
```ts
export async function listAutomations(
  opts: RpcOptions & { areaId?: string } = {},
): Promise<{ automations: Automation[] }> {
  const res = await rpcCall<unknown, { automations?: RawAutomation[] }>(
    `${AUTO_SVC}/List`,
    { areaId: opts.areaId },
    opts,
  );
  return { automations: (res.automations ?? []).map(decode) };
}
```

(Find the exact `AUTO_SVC` constant — if it's not named that, match the file's existing convention.)

- [ ] **Step 2: Typecheck**

Run: `cd app && npx vue-tsc --noEmit -p tsconfig.json 2>&1 | grep "automations\.ts"`
Expected: empty. If the existing `AutomationsView.vue` consumed the old return shape and now breaks, the TS error will surface here — fix accordingly (the new field is additive so this should be fine).

- [ ] **Step 3: Commit**

```bash
git add app/src/data/automations.ts
git commit -m "feat(app): listAutomations accepts areaId; exposes areaIds"
```

---

## Task 3.7: Wire Automations section in RoomDetailView + sample automation

**Files:**
- Modify: `app/src/views/RoomDetailView.vue`
- Add or modify: a Pkl file under `~/.local/share/switchyard/config/` declaring a sample automation tagged with `areas = ["<some-room-id>"]`

- [ ] **Step 1: Pick or create a sample automation**

Locate the dev Pkl config directory: `~/.local/share/switchyard/config/`.
List its `.pkl` files and find or create one declaring an automation tagged with a room. Example minimal addition (in a new or existing automations file):
```pkl
import "package://pkg.pkl-lang.org/your-pkg/.../automations.pkl" as auto
import "package://...trigger.pkl" as trig
import "package://...action.pkl" as act

local sampleAutomation = new auto.Automation {
  id = "bedroom-evening-test"
  triggers = new { ... }
  actions = new { ... }
  areas = new { "bedroom" }  // or whatever real area id
}
```

If declaring a fully-functional automation is too involved, simpler approach: pick an existing automation in the dev config and add `areas = new { "bedroom" }` to it. The point is to have at least one automation with a non-empty `areas` for E2E validation.

After editing, restart the daemon so the new config is loaded:
```bash
pkill -TERM -f "dist/switchyardd"; sleep 2; rm -f ~/.local/share/switchyard/switchyardd.lock
( source ~/.local/share/switchyard/config/secrets.env && exec ./dist/switchyardd ) > /tmp/switchyardd.log 2>&1 &
sleep 5
```

Verify: `curl -s --unix-socket ~/.local/share/switchyard/switchyardd.sock -H 'Content-Type: application/json' -H 'Connect-Protocol-Version: 1' -d '{}' http://localhost/switchyard.v1alpha1.AutomationService/List | python3 -c "import sys,json; d=json.load(sys.stdin); [print(a.get('id'), a.get('areaIds')) for a in d.get('automations',[])]"`
Expected: at least one line shows your automation with the area id.

- [ ] **Step 2: Replace the placeholder section in RoomDetailView**

In the script:
```ts
import { listAutomations, enableAutomation, disableAutomation, triggerAutomation, type Automation } from "@/data/automations";
import { SyAutomationCard } from "@/lib";

const automations = ref<Automation[]>([]);
const automationsLoading = ref<boolean>(true);
const automationsError = ref<string>("");
const autoActionError = ref<string>("");

async function loadAutomations(): Promise<void> {
  automationsLoading.value = true;
  automationsError.value = "";
  try {
    const r = await listAutomations({ areaId: areaId.value });
    automations.value = r.automations;
  } catch (err) {
    automationsError.value = err instanceof Error ? err.message : String(err);
  } finally { automationsLoading.value = false; }
}

async function refreshAutomations(): Promise<void> {
  try {
    const r = await listAutomations({ areaId: areaId.value });
    automations.value = r.automations;
  } catch { /* next refresh retries */ }
}

async function onToggleAutomation(a: Automation, next: boolean): Promise<void> {
  const idx = automations.value.findIndex((x) => x.id === a.id);
  if (idx === -1) return;
  const prev = automations.value[idx];
  automations.value[idx] = { ...prev, enabled: next };
  autoActionError.value = "";
  try {
    if (next) await enableAutomation(a.id);
    else      await disableAutomation(a.id);
    await refreshAutomations();
  } catch (err) {
    automations.value[idx] = prev;
    autoActionError.value = err instanceof Error ? err.message : String(err);
  }
}

async function onAutomationMenu(a: Automation, id: string): Promise<void> {
  autoActionError.value = "";
  try {
    if (id === "run") {
      await triggerAutomation(a.id);
      await refreshAutomations();
    } else {
      autoActionError.value = `${id} isn't wired yet`;
    }
  } catch (err) {
    autoActionError.value = err instanceof Error ? err.message : String(err);
  }
}

onMounted(() => {
  // existing void calls…
  void loadAutomations();
});
```

In the template, replace the Automations placeholder with:
```vue
<section class="page__section">
  <SyText variant="title" weight="semibold">Automations</SyText>

  <SySurface v-if="automationsLoading" padding="none">
    <SyEmptyState loading title="Loading automations…" />
  </SySurface>

  <SySurface v-else-if="automationsError" padding="none">
    <SyEmptyState
      intent="bad"
      title="Couldn't load automations"
      :description="automationsError"
    >
      <template #icon><SyIcon name="close" :size="28" /></template>
      <template #actions>
        <SyButton intent="secondary" @click="loadAutomations">Retry</SyButton>
      </template>
    </SyEmptyState>
  </SySurface>

  <SySurface v-else-if="automations.length === 0" padding="none">
    <SyEmptyState
      size="compact"
      title="No automations declared for this room"
      description="Add `areas = [\&quot;<id>\&quot;]` to an automation in your Pkl config."
    />
  </SySurface>

  <SySurface v-else padding="none" class="page__list">
    <SyAutomationCard
      v-for="a in automations"
      :key="a.id"
      :name="a.displayName"
      :trigger="a.mode || 'manual'"
      :enabled="a.enabled"
      :running="a.inFlight > 0"
      @toggle-enabled="(v) => onToggleAutomation(a, v)"
      @menu-action="(id) => onAutomationMenu(a, id)"
    />
  </SySurface>

  <SyText
    v-if="autoActionError"
    variant="caption"
    tone="bad"
    class="page__actionError"
  >
    {{ autoActionError }}
  </SyText>
</section>
```

- [ ] **Step 3: Typecheck**

Run: `cd app && npx vue-tsc --noEmit -p tsconfig.json 2>&1 | grep "RoomDetailView"`
Expected: empty.

- [ ] **Step 4: Playwright validation**

Open `http://localhost:5174/rooms/<the-room-id-you-tagged>`. Verify the Automations section shows the sample automation. Toggle its enabled flag, confirm no error, then refresh and confirm the change persisted (or reverted with an inline error if the daemon rejected). Take screenshot `room-detail-iter3.png`.

Visit `/rooms/<a-different-room>` — verify the Automations section is empty (or only shows automations tagged with that room).

- [ ] **Step 5: Commit**

```bash
git add app/src/views/RoomDetailView.vue
git commit -m "feat(app): RoomDetailView Automations section — area-scoped list + controls"
```

(The sample-automation Pkl edit lives in `~/.local/share/switchyard/config/`, which is gitignored. If the dev config should be shipped, that's a separate decision — flag it to the user before committing into the repo.)

---

## Task 3.8: Final cross-page validation

- [ ] **Step 1: Tour all pages**

Visit /, /rooms, /rooms/<id>, /activity, /automations, /devices, /settings. Confirm:
- No new console errors.
- Entity store still healthy.
- Global Automations page (`/automations`) still works — listAutomations without an areaId still returns everything.

- [ ] **Step 2: Run full Go test suite**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 3: Run full TS typecheck**

Run: `cd app && npx vue-tsc --noEmit -p tsconfig.json`
Expected: only pre-existing errors (the DriverStateName/DriverState mismatch + "overline" SyText variant — both predate this work). No new errors from this iteration.

- [ ] **Step 4: Verify no stray screenshots / probes leaked into git**

Run: `git status --short`
Expected: clean. Iteration screenshots (`room-detail-iter*.png`) are gitignored at the repo root.

- [ ] **Step 5: Iteration 3 complete**

The room detail dashboard is now fully wired: header + summary + scenes + entities (with controls) + activity (area-scoped) + automations (area-scoped). Three iterations, each independently shippable, all merged on `feat/new-ui`.

---

## Self-review notes

**Spec coverage:**
- Header + summary band → Task 1.4 (RoomDetailView template).
- Scenes row → Tasks 1.1, 1.2, 1.4.
- Entities section grouped by type → Task 1.4 (`entitiesByKind` computed).
- Activity section → Tasks 2.1–2.4.
- Automations section → Tasks 3.1–3.7.
- Make `/rooms` tiles clickable → Task 1.3.
- Backend B1 (StoriesFilter.entity_ids) → Tasks 2.1, 2.2.
- Backend B2 (Pkl→engine→proto Areas) → Tasks 3.1–3.5.
- Unknown room empty state → Task 1.4 (`isUnknownRoom`).
- Empty area whole-page empty state — handled implicitly: each section has its own empty state, so a room with no entities/scenes/activity/automations shows a stack of "no X" rather than one big one. **Spec says "show one big empty state instead of five empty sections" — that's a deviation.** Marking as a follow-up polish; not blocking.

**Placeholder scan:** Task 2.4 Step 2 has a deliberate `<pre>` placeholder with a stop-and-look note pointing the executor at `ActivityView.vue` for the right component. That's intentional — the alternative would be to invent a presentation that diverges from the global Activity page.

**Type consistency:** `entityIds` (camel) used consistently in TS data layer. `areaIds` (camel) on Automation. `area_id` (snake) on the wire because that's what the proto generates. Matches existing pattern in entities.ts (camel everywhere on the TS side).

**Unresolved decisions:**
- Whether the sample automation tagged with `areas = ["..."]` for iteration 3's E2E test should be committed to the repo (in `examples/automations/`) or just live in the dev config (`~/.local/share/switchyard/config/`). Task 3.7 Step 5 flags this for the user.
