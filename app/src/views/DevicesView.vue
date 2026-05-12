<!--
  DevicesView — list of installed drivers + driver-detail rail.

  Branches:
    - loading: SyEmptyState with spinner.
    - error: SyEmptyState in `bad` intent with a Retry action.
    - empty (loaded, zero drivers): "No drivers installed" CTA.
    - success: header summary + a grouped list of driver rows. Clicking
      a row opens a right-rail SySheet containing SyDriverPanel for the
      selected driver. Restart/stop actions fire RPCs and refresh.

  Auth: this view assumes the request is authenticated. In dev the Vite
  proxy connects over the daemon's UDS, so peercred authenticates the
  local user automatically (see app/vite.config.ts).
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import {
  SyText, SyListRow, SyDot, SyBadge, SyIcon, SyEmptyState, SyButton,
  SySurface, SySheet, SyDriverPanel,
} from "@/lib";
import {
  listDrivers, restartDriver,
  type DriverSummary,
} from "@/data/driver-management";
import { presentDriverState } from "@/data/driver-state";
import type { DriverInfo } from "@/lib/components/driver-panel/SyDriverPanel.vue";

type LoadState = "loading" | "ok" | "error";

const drivers = ref<DriverSummary[]>([]);
const state = ref<LoadState>("loading");
const errorMessage = ref<string>("");

/** Currently-selected driver id (the one whose rail is open), or null. */
const selectedId = ref<string | null>(null);
const sheetOpen = ref(false);
const actionBusy = ref(false);
const actionError = ref<string>("");

let abort: AbortController | null = null;

/** Initial / user-triggered load. Switches the page to a loading state
    so the empty-state flash doesn't appear while the request is in
    flight. Use `refresh()` for background updates that should preserve
    whatever the user is currently looking at. */
async function load(): Promise<void> {
  abort?.abort();
  abort = new AbortController();
  state.value = "loading";
  errorMessage.value = "";
  try {
    const res = await listDrivers({ signal: abort.signal });
    drivers.value = res.running;
    state.value = "ok";
  } catch (err) {
    if ((err as Error).name === "AbortError") return;
    state.value = "error";
    errorMessage.value = err instanceof Error ? err.message : String(err);
  }
}

/** Silent re-fetch that keeps the current page state intact. Used while
    polling for a stable post-restart state — we don't want the row to
    blink to "Loading" between ticks. Errors are swallowed: the next
    poll will retry, and a persistent failure leaves the existing list
    on screen rather than blowing the page up. */
async function refresh(): Promise<void> {
  try {
    const res = await listDrivers();
    drivers.value = res.running;
  } catch {
    /* Intentionally ignored — see comment above. */
  }
}

onMounted(load);
onBeforeUnmount(() => abort?.abort());

const summary = computed(() => {
  const list = drivers.value;
  const total = list.reduce((n, d) => n + d.entityCount, 0);
  const driversWord = list.length === 1 ? "driver" : "drivers";
  const entitiesWord = total === 1 ? "entity" : "entities";
  return `${list.length} ${driversWord} · ${total} ${entitiesWord}`;
});

/** Selected driver lookup. Null when no row is selected or the row is
    stale (was removed from the list after a refresh). */
const selectedDriver = computed<DriverSummary | null>(() => {
  if (!selectedId.value) return null;
  return drivers.value.find((d) => d.id === selectedId.value) ?? null;
});

/** Adapt a DriverSummary into SyDriverPanel's DriverInfo shape. */
const selectedDriverInfo = computed<DriverInfo | null>(() => {
  const d = selectedDriver.value;
  if (!d) return null;
  return {
    name: d.id,
    pack: d.pack,
    version: d.version,
    state: d.state,
    stateDetail: stateDetailFor(d),
    entityCount: d.entityCount,
    /* Entity-type breakdown isn't returned by List yet. Pulling per-driver
       Get with type breakdown is a follow-up; for now we render the count
       alone and let the panel hide the types block when the array is
       absent. */
    entityTypes: undefined,
  };
});

function stateDetailFor(d: DriverSummary): string | undefined {
  if (d.state === "running" && d.uptimeSeconds > 0) {
    return `Up for ${formatDuration(d.uptimeSeconds)}`;
  }
  return undefined;
}

/** Human-format seconds → "4h 12m" / "12m 4s" / "47s". */
function formatDuration(s: number): string {
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = Math.floor(s % 60);
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${sec}s`;
  return `${sec}s`;
}

function openDriver(d: DriverSummary): void {
  selectedId.value = d.id;
  sheetOpen.value = true;
  actionError.value = "";
}

function onSheetClose(open: boolean): void {
  sheetOpen.value = open;
  if (!open) actionError.value = "";
}

async function onRestart(): Promise<void> {
  const id = selectedId.value;
  if (!id) return;
  actionBusy.value = true;
  actionError.value = "";
  try {
    await restartDriver(id, "User initiated from UI");
    /* A restart fires a sequence of transient states (spawning →
       reconnecting → running). One immediate refresh would catch a
       transient state and leave it stuck on screen, so we poll until
       the driver returns to a terminal state or we hit the budget. */
    await pollUntilStable(id, { intervalMs: 750, timeoutMs: 12_000 });
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : String(err);
  } finally {
    actionBusy.value = false;
  }
}

/** Wait until the given driver reaches a non-transient state, refreshing
    the list on each tick. Resolves on stable state or on timeout — never
    rejects (we'd rather show a stale "reconnecting" row than break the
    page). */
async function pollUntilStable(
  id: string,
  opts: { intervalMs: number; timeoutMs: number },
): Promise<void> {
  const deadline = Date.now() + opts.timeoutMs;
  while (Date.now() < deadline) {
    await refresh();
    const d = drivers.value.find((x) => x.id === id);
    /* Terminal states from the daemon's point of view. `unknown` means the
       supervisor doesn't know about the driver — we treat that as terminal
       so we don't poll forever on an instance that's been deleted. */
    if (!d || d.state === "running" || d.state === "stopped" ||
        d.state === "degraded" || d.state === "unknown") {
      return;
    }
    await new Promise((r) => setTimeout(r, opts.intervalMs));
  }
}

function onConfigure(): void {
  /* TODO: route to /settings/pkl-config with the driver's block selected. */
}

function onViewLogs(): void {
  /* TODO: open a SySheet with streaming driver logs. */
}
</script>

<template>
  <div class="page">
    <header class="page__head">
      <SyText as="h1" variant="display">Devices</SyText>
      <SyText v-if="state === 'ok'" variant="body" tone="subtle">
        {{ summary }}
      </SyText>
    </header>

    <SySurface v-if="state === 'loading'" padding="none">
      <SyEmptyState loading title="Loading drivers…" description="Talking to the daemon." />
    </SySurface>

    <SySurface v-else-if="state === 'error'" padding="none">
      <SyEmptyState
        intent="bad"
        title="Couldn't load drivers"
        :description="errorMessage"
      >
        <template #icon><SyIcon name="close" :size="28" /></template>
        <template #actions>
          <SyButton intent="secondary" @click="load">Retry</SyButton>
        </template>
      </SyEmptyState>
    </SySurface>

    <SySurface v-else-if="drivers.length === 0" padding="none">
      <SyEmptyState
        title="No drivers installed"
        description="Drivers are configured in Pkl. Add one to start controlling devices."
      >
        <template #icon><SyIcon name="plugin" :size="28" /></template>
        <template #actions>
          <SyButton intent="primary">Open Pkl config</SyButton>
        </template>
      </SyEmptyState>
    </SySurface>

    <SySurface v-else padding="none" class="page__list">
      <SyListRow
        v-for="d in drivers"
        :key="d.id"
        as="button"
        density="comfortable"
        interactive
        :bordered="false"
        @click="openDriver(d)"
      >
        <template #leading>
          <SyDot
            :intent="presentDriverState(d.state).intent"
            :pulse="presentDriverState(d.state).pulse"
          />
        </template>
        <SyText weight="medium">{{ d.id }}</SyText>
        <SyText variant="caption" tone="subtle" class="page__pack">
          {{ d.pack }}{{ d.version ? `@${d.version}` : "" }}
        </SyText>
        <template #trailing>
          <SyBadge
            :intent="presentDriverState(d.state).intent"
            :pulse="presentDriverState(d.state).pulse"
            dot
          >
            {{ presentDriverState(d.state).label }}
          </SyBadge>
          <SyText variant="caption" tone="subtle">
            {{ d.entityCount }} {{ d.entityCount === 1 ? "entity" : "entities" }}
          </SyText>
          <SyIcon name="chevron-right" :size="14" />
        </template>
      </SyListRow>
    </SySurface>

    <SySheet
      :modelValue="sheetOpen"
      side="right"
      size="md"
      :title="selectedDriver?.id ?? 'Driver'"
      @update:modelValue="onSheetClose"
    >
      <template v-if="selectedDriverInfo">
        <SyDriverPanel
          :driver="selectedDriverInfo"
          :busy="actionBusy"
          @restart="onRestart"
          @configure="onConfigure"
          @view-logs="onViewLogs"
        />
        <SyText
          v-if="actionError"
          variant="caption"
          tone="bad"
          class="page__actionError"
        >
          {{ actionError }}
        </SyText>
      </template>
    </SySheet>
  </div>
</template>

<style scoped>
.page {
  padding: var(--sy-space-5) var(--sy-space-6);
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-4);
  max-width: 1080px;
}
.page__head {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-1);
}

/* Hairline between sibling list-rows inside the grouped surface. */
.page__list :deep(.sy-listrow + .sy-listrow) {
  border-top: 1px solid var(--sy-color-line-soft);
}

.page__pack {
  font-family: var(--sy-font-numeric);
  font-feature-settings: var(--sy-numeric-feature);
}

.page__actionError {
  margin-top: var(--sy-space-3);
}
</style>
