<!--
  HomeView — landing dashboard.

  Three rows:
    1. Greeting + time-of-day-aware sub-line
    2. Four stat tiles: drivers running / entities / automations / rooms.
       Each tile is a router link into its dedicated page.
    3. Recent activity feed (top 5 events) + "View all" link to Activity.

  Each section loads independently with its own loading/error fallback
  so the page degrades gracefully when one service is unavailable
  (e.g., AutomationService not wired in dev).
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import {
  SyText, SySurface, SyButton, SyIcon, SyEmptyState, SyStatTile, SyEventRow,
} from "@/lib";
import { listDrivers, type DriverSummary } from "@/data/driver-management";
import { listAutomations, type Automation } from "@/data/automations";
import { listAreas } from "@/data/areas";
import { listEvents, type EventRecord } from "@/data/activity";
import { presentEvent } from "@/data/event-display";
import { entityStore } from "@/stores/entity-store";

/* ---- Greeting -------------------------------------------------------- */
const tickNow = ref<Date>(new Date());
let tickHandle: number | null = null;

const greeting = computed<string>(() => {
  const h = tickNow.value.getHours();
  if (h < 5)  return "Up late";
  if (h < 12) return "Good morning";
  if (h < 18) return "Good afternoon";
  if (h < 22) return "Good evening";
  return "Good night";
});

/* ---- Stats: drivers ------------------------------------------------- */
const drivers = ref<DriverSummary[]>([]);
const driversLoading = ref<boolean>(true);
const driversRunning = computed<number>(() => drivers.value.filter((d) => d.state === "running").length);
const driversDetail = computed<string>(() => {
  const t = drivers.value.length;
  return t === 1 ? "of 1 driver" : `of ${t} drivers`;
});

/* ---- Stats: entities ------------------------------------------------ */
/* Reads from the global entity store. Loading is bound to !hydrated for
   the first connection only; once we have a count, we don't revert to
   "loading" on a transient stream disconnect (the dashboard would look
   broken). */
const entityCount = computed<number>(() => entityStore.entities.value.size);
const entitiesLoading = computed<boolean>(() => !entityStore.hydrated.value);

/* ---- Stats: automations --------------------------------------------- */
const automations = ref<Automation[]>([]);
const automationsLoading = ref<boolean>(true);
const automationsEnabled = computed<number>(() => automations.value.filter((a) => a.enabled).length);
const automationsDetail = computed<string>(() => {
  const t = automations.value.length;
  if (t === 0) return "none yet";
  return t === 1 ? "of 1 enabled" : `of ${t} enabled`;
});

/* ---- Stats: rooms (areas) ------------------------------------------- */
const areaCount = ref<number>(0);
const areasLoading = ref<boolean>(true);

/* ---- Recent activity ------------------------------------------------ */
const recentEvents = ref<EventRecord[]>([]);
const recentLoading = ref<boolean>(true);
const recentError = ref<string>("");

const router = useRouter();
const aborts: AbortController[] = [];

function newAbort(): AbortController {
  const a = new AbortController();
  aborts.push(a);
  return a;
}

async function loadDrivers(): Promise<void> {
  const ac = newAbort();
  driversLoading.value = true;
  try {
    const r = await listDrivers({ signal: ac.signal });
    drivers.value = r.running;
  } catch { /* swallow: tile renders dash */ }
  finally { driversLoading.value = false; }
}
async function loadAutomations(): Promise<void> {
  const ac = newAbort();
  automationsLoading.value = true;
  try {
    const r = await listAutomations({ signal: ac.signal });
    automations.value = r.automations;
  } catch { /* swallow */ }
  finally { automationsLoading.value = false; }
}
async function loadAreas(): Promise<void> {
  const ac = newAbort();
  areasLoading.value = true;
  try {
    const r = await listAreas({ signal: ac.signal });
    areaCount.value = r.areas.length;
  } catch { /* swallow */ }
  finally { areasLoading.value = false; }
}
async function loadRecent(): Promise<void> {
  const ac = newAbort();
  recentLoading.value = true;
  recentError.value = "";
  try {
    const r = await listEvents({}, { signal: ac.signal });
    /* Server returns newest-first across the 24h default window;
       take the first 5 for the dashboard preview. */
    recentEvents.value = r.events.slice(0, 5);
  } catch (err) {
    recentError.value = err instanceof Error ? err.message : String(err);
  } finally {
    recentLoading.value = false;
  }
}

onMounted(() => {
  void loadDrivers();
  void loadAutomations();
  void loadAreas();
  void loadRecent();
  tickHandle = window.setInterval(() => { tickNow.value = new Date(); }, 60_000);
});

onBeforeUnmount(() => {
  for (const a of aborts) a.abort();
  if (tickHandle !== null) window.clearInterval(tickHandle);
});

function openEvent(id: string): void {
  /* Route to the Activity page with the event deep-linked open. */
  router.push(`/activity?tab=events&event=${encodeURIComponent(id)}`);
}
</script>

<template>
  <div class="page">
    <header class="page__head">
      <SyText as="h1" variant="display">{{ greeting }}</SyText>
      <SyText variant="body" tone="subtle">
        Here's what's happening across Switchyard right now.
      </SyText>
    </header>

    <section class="page__stats">
      <SyStatTile
        icon="plugin"
        intent="info"
        label="Drivers"
        :value="driversRunning"
        :detail="driversDetail"
        :loading="driversLoading"
        to="/devices"
      />
      <SyStatTile
        icon="bulb"
        intent="accent"
        label="Entities"
        :value="entityCount"
        detail="registered"
        :loading="entitiesLoading"
      />
      <SyStatTile
        icon="automations"
        intent="automation"
        label="Automations"
        :value="automationsEnabled"
        :detail="automationsDetail"
        :loading="automationsLoading"
        to="/automations"
      />
      <SyStatTile
        icon="rooms"
        intent="good"
        label="Rooms"
        :value="areaCount"
        :detail="areaCount === 1 ? 'room' : 'rooms'"
        :loading="areasLoading"
        to="/rooms"
      />
    </section>

    <section class="page__activity">
      <div class="page__sectionHead">
        <SyText variant="title" weight="semibold">Recent activity</SyText>
        <SyButton intent="ghost" size="sm" @click="router.push('/activity')">
          View all
          <SyIcon name="chevron-right" :size="12" />
        </SyButton>
      </div>

      <SySurface v-if="recentLoading" padding="none">
        <SyEmptyState loading title="Loading recent events…" />
      </SySurface>

      <SySurface v-else-if="recentError" padding="none">
        <SyEmptyState
          intent="bad"
          title="Couldn't load activity"
          :description="recentError"
        >
          <template #icon><SyIcon name="close" :size="28" /></template>
          <template #actions>
            <SyButton intent="secondary" @click="loadRecent">Retry</SyButton>
          </template>
        </SyEmptyState>
      </SySurface>

      <SySurface v-else-if="recentEvents.length === 0" padding="none">
        <SyEmptyState
          title="Quiet over here"
          description="No events in the last 24 hours."
        >
          <template #icon><SyIcon name="activity" :size="28" /></template>
        </SyEmptyState>
      </SySurface>

      <SySurface v-else padding="none" class="page__list">
        <SyEventRow
          v-for="e in recentEvents"
          :key="e.eventId"
          interactive
          v-bind="presentEvent(e, tickNow)"
          @click="openEvent(e.eventId)"
        />
      </SySurface>
    </section>
  </div>
</template>

<style scoped>
.page {
  padding: var(--sy-space-5) var(--sy-space-6);
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-5);
  max-width: 1080px;
}
.page__head {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-1);
}

.page__stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: var(--sy-space-3);
}

.page__activity {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-3);
}
.page__sectionHead {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.page__list :deep(.sy-listrow + .sy-listrow) {
  border-top: 1px solid var(--sy-color-line-soft);
}
</style>
