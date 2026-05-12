<!--
  RoomDetailView — the per-room dashboard at /rooms/:id.

  Sections (in order):
    1. Header: room name, entity count, on/off summary
    2. Scenes: chip row, click to apply (suppressed when SceneService
       is unimplemented; surfaces real errors otherwise)
    3. Entities: SyEntityRow grouped by type
    4. Activity: deferred to Iteration 2 (placeholder empty state)
    5. Automations: deferred to Iteration 3 (placeholder empty state)

  Reads entity state live via entityStore.byArea. Scenes are listed
  globally — there's no per-area scene scoping in the proto today.
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  SyText, SySurface, SyButton, SyIcon, SyEmptyState,
  SyEntityRow, SyScene, SyStoryRow,
} from "@/lib";
import { listAreas, type Area } from "@/data/areas";
import { listScenes, applyScene, type Scene } from "@/data/scenes";
import { listStories, type Story } from "@/data/activity";
import { formatEventTimestamp } from "@/data/event-display";
import { entityStore } from "@/stores/entity-store";
import { RpcError } from "@/data/rpc";
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
/** True when the daemon returned 501/Unimplemented for SceneService.List.
 *  We suppress the whole section in that case rather than surfacing a
 *  noisy error — scenes are simply not available yet. */
const scenesUnimplemented = ref<boolean>(false);
const scenesBusy = ref<Set<string>>(new Set());
const sceneError = ref<string>("");

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
  scenesUnimplemented.value = false;
  try {
    const r = await listScenes();
    scenes.value = r.scenes;
  } catch (err) {
    if (err instanceof RpcError && err.status === 501) {
      scenesUnimplemented.value = true;
    } else {
      scenesError.value = err instanceof Error ? err.message : String(err);
    }
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

const showScenesSection = computed<boolean>(() =>
  !scenesUnimplemented.value && (scenesLoading.value || scenes.value.length > 0 || !!scenesError.value));

/* ---- Recent activity (scoped to this area's entities) -------------- */
const stories = ref<Story[]>([]);
const storiesLoading = ref<boolean>(true);
const storiesError = ref<string>("");
const tickNow = ref<Date>(new Date());
let tickHandle: number | null = null;

async function loadStories(): Promise<void> {
  storiesLoading.value = true;
  storiesError.value = "";
  try {
    const ids = entities.value.map((e) => e.id);
    if (ids.length === 0) {
      stories.value = [];
      return;
    }
    const r = await listStories({ filter: { entityIds: ids } });
    stories.value = r.stories.slice(0, 5);
  } catch (err) {
    storiesError.value = err instanceof Error ? err.message : String(err);
  } finally {
    storiesLoading.value = false;
  }
}

/** Color the story icon by its top tag category — same mapping as
 *  ActivityView so the two pages read consistently. */
function storyPresentation(s: Story): {
  icon: "sparkle" | "alert" | "activity"; intent: "automation" | "warn" | "info";
} {
  const cat = s.tags[0]?.category ?? "";
  if (cat === "failure" || cat === "security") return { icon: "alert",   intent: "warn"       };
  if (cat === "causation")                     return { icon: "sparkle", intent: "automation" };
  return { icon: "activity", intent: "info" };
}

onMounted(() => {
  void loadAreas();
  void loadScenes();
  void loadStories();
  // Keep relative timestamps fresh.
  tickHandle = window.setInterval(() => { tickNow.value = new Date(); }, 60_000);
});
onBeforeUnmount(() => {
  if (tickHandle !== null) window.clearInterval(tickHandle);
});

// Whenever the entity-set for this area materially changes, refresh stories.
watch(() => entities.value.map((e) => e.id).join(","), (s) => {
  if (s !== "") void loadStories();
});

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
          {{ entities.length === 1 ? "entity" : "entities" }}<template v-if="entities.length > 0">
            · {{ onCount }} on / {{ offCount }} off
          </template>
        </SyText>
      </header>

      <!-- Scenes (suppressed when SceneService is unimplemented) -->
      <section v-if="showScenesSection" class="page__section">
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

      <!-- Activity (scoped to this area's entities) -->
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
          <SyStoryRow
            v-for="s in stories"
            :key="s.id"
            :icon="storyPresentation(s).icon"
            :intent="storyPresentation(s).intent"
            :title="s.title || 'Story'"
            :meta="s.entityIds.length ? s.entityIds.slice(0, 3).join(' · ') : s.source"
            :count="s.innerEventIds.length > 1 ? s.innerEventIds.length : 0"
            :timestamp="formatEventTimestamp(s.occurredAt, tickNow)"
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
