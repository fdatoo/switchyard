<!--
  RoomsView — grid of areas (rooms) the user has configured.

  Each tile shows the room name, an entity count, and a thin badge
  stripe with per-type counts. Clicking a tile (eventually) opens a
  per-room detail view; today the tile is non-interactive — we can
  add `href` once we have a /rooms/:id route.

  Empty / loading / error states mirror Devices and Activity for
  cross-page consistency.
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import {
  SyText, SySurface, SyButton, SyIcon, SyEmptyState, SyRoomTile, SyBadge,
} from "@/lib";
import { listAreas, type Area } from "@/data/areas";
import { listEntities, type Entity } from "@/data/entities";

type LoadState = "loading" | "ok" | "error";

const areas = ref<Area[]>([]);
const entities = ref<Entity[]>([]);
const state = ref<LoadState>("loading");
const errorMessage = ref<string>("");

let abort: AbortController | null = null;

async function load(): Promise<void> {
  abort?.abort();
  abort = new AbortController();
  state.value = "loading";
  errorMessage.value = "";
  try {
    /* Areas and entities load in parallel — neither depends on the
       other and they're served by different RPCs, so a single AbortSignal
       cancels both. */
    const [aRes, eRes] = await Promise.all([
      listAreas({ signal: abort.signal }),
      listEntities({ signal: abort.signal }),
    ]);
    areas.value = aRes.areas;
    entities.value = eRes.entities;
    state.value = "ok";
  } catch (err) {
    if ((err as Error).name === "AbortError") return;
    state.value = "error";
    errorMessage.value = err instanceof Error ? err.message : String(err);
  }
}

onMounted(load);
onBeforeUnmount(() => abort?.abort());

/** Per-area entity index. Computed once per render so each tile is O(1)
    on lookup; the underlying data sets are small. Entities with no
    area land in the synthetic `_unassigned` bucket. */
const entitiesByArea = computed<Map<string, Entity[]>>(() => {
  const m = new Map<string, Entity[]>();
  for (const e of entities.value) {
    const key = e.areaId || "_unassigned";
    const list = m.get(key) ?? [];
    list.push(e);
    m.set(key, list);
  }
  return m;
});

/** Per-area type breakdown: { light: 3, sensor: 1 }. Drives the badge stripe. */
function typeCounts(areaId: string): Record<string, number> {
  const out: Record<string, number> = {};
  for (const e of entitiesByArea.value.get(areaId) ?? []) {
    out[e.type] = (out[e.type] ?? 0) + 1;
  }
  return out;
}

function entityCount(areaId: string): number {
  return entitiesByArea.value.get(areaId)?.length ?? 0;
}

const orphanCount = computed<number>(() => entitiesByArea.value.get("_unassigned")?.length ?? 0);
</script>

<template>
  <div class="page">
    <header class="page__head">
      <SyText as="h1" variant="display">Rooms</SyText>
      <SyText v-if="state === 'ok'" variant="body" tone="subtle">
        {{ areas.length }} {{ areas.length === 1 ? "room" : "rooms" }}<template v-if="orphanCount > 0">
          · {{ orphanCount }} unassigned entit{{ orphanCount === 1 ? "y" : "ies" }}
        </template>
      </SyText>
    </header>

    <SySurface v-if="state === 'loading'" padding="none">
      <SyEmptyState loading title="Loading rooms…" />
    </SySurface>

    <SySurface v-else-if="state === 'error'" padding="none">
      <SyEmptyState
        intent="bad"
        title="Couldn't load rooms"
        :description="errorMessage"
      >
        <template #icon><SyIcon name="close" :size="28" /></template>
        <template #actions>
          <SyButton intent="secondary" @click="load">Retry</SyButton>
        </template>
      </SyEmptyState>
    </SySurface>

    <SySurface v-else-if="areas.length === 0" padding="none">
      <SyEmptyState
        title="No rooms yet"
        description="Rooms (areas) are declared in Pkl config. Add one to start grouping devices."
      >
        <template #icon><SyIcon name="rooms" :size="28" /></template>
        <template #actions>
          <SyButton intent="primary">Open Pkl config</SyButton>
        </template>
      </SyEmptyState>
    </SySurface>

    <div v-else class="page__grid">
      <SyRoomTile
        v-for="a in areas"
        :key="a.id"
        :name="a.displayName"
        :meta="`${entityCount(a.id)} ${entityCount(a.id) === 1 ? 'entity' : 'entities'}`"
      >
        <SyBadge
          v-for="(count, type) in typeCounts(a.id)"
          :key="type"
          intent="neutral"
          size="sm"
        >
          {{ count }} {{ type }}{{ count === 1 ? "" : "s" }}
        </SyBadge>
        <SyText
          v-if="entityCount(a.id) === 0"
          variant="caption"
          tone="subtle"
        >
          No entities assigned
        </SyText>
      </SyRoomTile>
    </div>
  </div>
</template>

<style scoped>
.page {
  padding: var(--sy-space-5) var(--sy-space-6);
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-4);
  max-width: 1200px;
}
.page__head {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-1);
}
.page__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: var(--sy-space-3);
}
</style>
