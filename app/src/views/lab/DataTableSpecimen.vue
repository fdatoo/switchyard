<script setup lang="ts">
import { computed, ref } from "vue";
import {
  SyText, SyDataTable, SyBadge,
  SySearchInput, SyFilterChip, SyMenu, SyIcon, SyTooltip,
} from "@/lib";
import type { ColumnDef, SortState } from "@/lib/components/datatable/types";
import type { MenuItem } from "@/lib/components/menu/types";

interface Driver {
  id: string;
  name: string;
  pack: string;
  state: "running" | "reconnecting" | "stopped";
  entities: number;
}

const allDrivers: Driver[] = [
  { id: "hue_main",      name: "hue_main",       pack: "switchyard/hue@2.4.1",     state: "running",      entities: 12 },
  { id: "matter_bridge", name: "matter_bridge",  pack: "switchyard/matter@1.0.3",  state: "reconnecting", entities: 4  },
  { id: "shelly_legacy", name: "shelly_legacy",  pack: "switchyard/shelly@0.9.1",  state: "stopped",      entities: 0  },
  { id: "lutron_caseta", name: "lutron_caseta",  pack: "switchyard/lutron@1.2.0",  state: "running",      entities: 7  },
  { id: "tuya_office",   name: "tuya_office",    pack: "switchyard/tuya@1.4.2",    state: "stopped",      entities: 3  },
];

const columns: ColumnDef<Driver>[] = [
  { id: "name",     header: "Name",     sortable: true },
  { id: "pack",     header: "Pack",     sortable: true },
  { id: "state",    header: "State",    sortable: true },
  { id: "entities", header: "Entities", sortable: true, align: "right", width: 100 },
];

const search = ref("");
const stateFilter = ref<Driver["state"] | null>(null);
const sort = ref<SortState | null>({ columnId: "name", direction: "asc" });

const filteredRows = computed(() => {
  let r = allDrivers;
  if (search.value) {
    const q = search.value.toLowerCase();
    r = r.filter((d) => d.name.includes(q) || d.pack.toLowerCase().includes(q));
  }
  if (stateFilter.value) {
    r = r.filter((d) => d.state === stateFilter.value);
  }
  if (sort.value) {
    const { columnId, direction } = sort.value;
    const dir = direction === "asc" ? 1 : -1;
    r = [...r].sort((a, b) => {
      const av = (a as Record<string, unknown>)[columnId];
      const bv = (b as Record<string, unknown>)[columnId];
      if (typeof av === "number" && typeof bv === "number") return (av - bv) * dir;
      return String(av).localeCompare(String(bv)) * dir;
    });
  }
  return r;
});

const stateMenu: MenuItem[] = [
  { type: "header", label: "Filter by state" },
  { type: "item", id: "running",      label: "Running"      },
  { type: "item", id: "reconnecting", label: "Reconnecting" },
  { type: "item", id: "stopped",      label: "Stopped"      },
];

function selectState(id: string): void {
  stateFilter.value = id as Driver["state"];
}

function clearState(): void {
  stateFilter.value = null;
}

const stateBadgeIntent = (s: Driver["state"]) =>
  s === "running" ? "good" : s === "reconnecting" ? "warn" : "neutral";
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Drivers · with filter toolbar</SyText>
      <SyDataTable :columns="columns" :rows="filteredRows" v-model:sort="sort" emptyTitle="No matches">
        <template #toolbar>
          <SySearchInput v-model="search" size="sm" placeholder="Search drivers…" />
          <SyFilterChip
            v-if="stateFilter"
            field="state"
            :label="stateFilter"
            :intent="stateBadgeIntent(stateFilter)"
            @remove="clearState"
          />
          <SyMenu :items="stateMenu" @select="selectState">
            <template #trigger>
              <SyTooltip content="Add filter">
                <button type="button" class="iconbtn" aria-label="Add filter">
                  <SyIcon name="filter" :size="14" />
                </button>
              </SyTooltip>
            </template>
          </SyMenu>
          <span class="spacer" />
          <SyText variant="caption" tone="subtle">
            {{ filteredRows.length }} of {{ allDrivers.length }}
          </SyText>
        </template>
        <template #cell-state="{ row }">
          <SyBadge
            :intent="stateBadgeIntent(row.state)"
            :pulse="row.state === 'running' ? 'slow' : row.state === 'reconnecting' ? 'fast' : 'off'"
            dot
          >
            {{ row.state }}
          </SyBadge>
        </template>
      </SyDataTable>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">SySearchInput · standalone</SyText>
      <SySearchInput v-model="search" placeholder="Find an entity, driver, or page…" />
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">SyFilterChip · intents</SyText>
      <div class="chips">
        <SyFilterChip field="state" label="running" intent="good" />
        <SyFilterChip field="state" label="reconnecting" intent="warn" />
        <SyFilterChip field="driver" label="hue_main" intent="accent" />
        <SyFilterChip label="last 7 days" />
        <SyFilterChip field="state" label="stopped" intent="bad" :removable="false" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.chips { display: flex; flex-wrap: wrap; gap: var(--sy-space-2); align-items: center; }
.spacer { flex: 1; }
.iconbtn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  background: transparent;
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-sm);
  color: var(--sy-color-fg-3);
  cursor: pointer;
  transition: background var(--sy-motion-fast), color var(--sy-motion-fast), border-color var(--sy-motion-fast);
}
.iconbtn:hover {
  background: var(--sy-color-surface-2);
  color: var(--sy-color-fg);
  border-color: var(--sy-color-fg-5);
}
</style>
