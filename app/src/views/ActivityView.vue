<!--
  ActivityView — the Activity page.

  Three big features stack on the tier-1/2 primitives:

  1. **Live tail** — a 5s background poll asks for events with sequence
     greater than the highest we've seen and prepends them. A "Live"
     dot pulses while polling and ticks when new rows land.

  2. **Filters** — free-text (client-side over what's loaded), time
     window (server-side via `since`), interesting-only (client-side),
     plus three facets (kind / source / entity) rendered as removable
     SyFilterChip's. Facets are added by clicking the matching field
     in the event detail rail or, for kind, in the row's badge stripe.

  3. **Detail rail** — clicking an event opens a SySheet with the full
     record: rich title, interestingness tags, identity fields, parsed
     payload, and the causation chain (ancestors). Stories expand
     in-place to show their constituent events.

  Saved queries (third tab in the v2 vision) are still deferred — when
  we add it, this file already has the filter state we need to seed the
  Save form.
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  SyText, SySurface, SyTabs, SyEmptyState, SyButton, SyIcon,
  SySearchInput, SySegmented, SyEventRow, SyStoryRow, SySheet, SyFilterChip,
  SyDot, SySwitch,
} from "@/lib";
import {
  listEvents, listStories,
  type EventRecord, type Story, type EventsFilter,
} from "@/data/activity";
import { presentEvent, formatEventTimestamp } from "@/data/event-display";
import EventDetailContent from "./activity/EventDetailContent.vue";

type TabId = "stories" | "events";
type LoadState = "loading" | "ok" | "error";
type WindowId = "1h" | "24h" | "7d" | "all";
type TagCategory = "" | "failure" | "performance" | "causation" | "anomaly" | "security" | "configuration" | "novelty";

interface WindowOption {
  id: WindowId;
  label: string;
  since: (now: Date) => Date | undefined;
}

const WINDOWS: readonly WindowOption[] = [
  { id: "1h",  label: "Last hour",   since: (n) => new Date(n.getTime() - 3.6e6) },
  { id: "24h", label: "Last 24h",    since: (n) => new Date(n.getTime() - 24 * 3.6e6) },
  { id: "7d",  label: "Last 7 days", since: (n) => new Date(n.getTime() - 7 * 24 * 3.6e6) },
  { id: "all", label: "All time",    since: () => undefined },
] as const;

const LIVE_POLL_MS = 5_000;

/* ---- UI state -------------------------------------------------------- */
const route = useRoute();
const router = useRouter();

/* All filter state is hydrated from the URL on mount so refreshing or
   sharing the link preserves the view. Each ref's initial value comes
   from `?tab=…&win=…&q=…&kind=…&source=…&entity=…&interesting=1`. */
function qstr(key: string, fallback: string): string {
  const v = route.query[key];
  return typeof v === "string" ? v : fallback;
}
function qbool(key: string): boolean {
  return route.query[key] === "1";
}
function isTabId(v: string): v is TabId { return v === "stories" || v === "events"; }
function isWindowId(v: string): v is WindowId { return v === "1h" || v === "24h" || v === "7d" || v === "all"; }

const tab = ref<TabId>(isTabId(qstr("tab", "stories")) ? qstr("tab", "stories") as TabId : "stories");
const search = ref<string>(qstr("q", ""));
const windowId = ref<WindowId>(isWindowId(qstr("win", "24h")) ? qstr("win", "24h") as WindowId : "24h");
const interestingOnly = ref<boolean>(qbool("interesting"));
const kindFacet = ref<string>(qstr("kind", ""));
const sourceFacet = ref<string>(qstr("source", ""));
const entityFacet = ref<string>(qstr("entity", ""));

/* The URL-sync watch is registered later (after `selectedEventId` and
   `detailOpen` are declared) so its dependency list can include them. */

/* ---- Data ------------------------------------------------------------ */
const stories = ref<Story[]>([]);
const storiesCursor = ref<string>("");
const storiesState = ref<LoadState>("loading");
const storiesError = ref<string>("");
const expandedStories = ref<Set<string>>(new Set());

const events = ref<EventRecord[]>([]);
const eventsCursor = ref<string>("");
const eventsState = ref<LoadState>("loading");
const eventsError = ref<string>("");

const selectedEventId = ref<string>(qstr("event", ""));
const detailOpen = ref<boolean>(!!qstr("event", ""));

/* Push state into the URL whenever it changes. `replace` (not `push`)
   keeps Back behaving naturally — one history entry per "real" nav,
   not per keystroke in the search box. */
watch(
  [tab, search, windowId, interestingOnly, kindFacet, sourceFacet, entityFacet, selectedEventId, detailOpen],
  () => {
    const q: Record<string, string> = {};
    if (tab.value !== "stories") q.tab = tab.value;
    if (search.value)            q.q = search.value;
    if (windowId.value !== "24h") q.win = windowId.value;
    if (interestingOnly.value)   q.interesting = "1";
    if (kindFacet.value)         q.kind = kindFacet.value;
    if (sourceFacet.value)       q.source = sourceFacet.value;
    if (entityFacet.value)       q.entity = entityFacet.value;
    if (detailOpen.value && selectedEventId.value) q.event = selectedEventId.value;
    router.replace({ query: q });
  },
);

/* Highest event sequence currently in memory — drives the live-tail diff.
   Live calls don't reset eventsState, so the page never blinks. */
const highestSequence = computed<number>(() => {
  let max = 0;
  for (const e of events.value) if (e.sequence > max) max = e.sequence;
  return max;
});
const livePollActive = ref<boolean>(false);
const liveNewCount = ref<number>(0); // most-recent tick's new event count

let storiesAbort: AbortController | null = null;
let eventsAbort: AbortController | null = null;
let liveTimer: number | null = null;
let tickHandle: number | null = null;
const tickNow = ref<Date>(new Date());

/** Filter sent to the server. We push kind to the server because the
    daemon honors it; everything else is client-side. */
const serverFilter = computed<EventsFilter>(() => {
  const win = WINDOWS.find((w) => w.id === windowId.value) ?? WINDOWS[0];
  return {
    since: win.since(new Date()),
    kind: kindFacet.value || undefined,
  };
});

/* Window or kind change → full reload (different time bound / server filter). */
watch([windowId, kindFacet], () => {
  loadStories({ reset: true });
  loadEvents({ reset: true });
});

/* ---- Loaders --------------------------------------------------------- */

async function loadStories(opts: { reset: boolean }): Promise<void> {
  if (opts.reset) {
    storiesAbort?.abort();
    storiesAbort = new AbortController();
    storiesState.value = "loading";
    storiesError.value = "";
  }
  try {
    const res = await listStories(
      { filter: serverFilter.value, cursor: opts.reset ? "" : storiesCursor.value },
      { signal: storiesAbort?.signal },
    );
    stories.value = opts.reset ? res.stories : [...stories.value, ...res.stories];
    storiesCursor.value = res.nextCursor;
    storiesState.value = "ok";
  } catch (err) {
    if ((err as Error).name === "AbortError") return;
    storiesState.value = "error";
    storiesError.value = err instanceof Error ? err.message : String(err);
  }
}

async function loadEvents(opts: { reset: boolean }): Promise<void> {
  if (opts.reset) {
    eventsAbort?.abort();
    eventsAbort = new AbortController();
    eventsState.value = "loading";
    eventsError.value = "";
  }
  try {
    const res = await listEvents(
      { filter: serverFilter.value, cursor: opts.reset ? "" : eventsCursor.value },
      { signal: eventsAbort?.signal },
    );
    events.value = opts.reset ? res.events : [...events.value, ...res.events];
    eventsCursor.value = res.nextCursor;
    eventsState.value = "ok";
  } catch (err) {
    if ((err as Error).name === "AbortError") return;
    eventsState.value = "error";
    eventsError.value = err instanceof Error ? err.message : String(err);
  }
}

/** Background poll: fetch the most-recent slice in the current window
    and prepend anything with a higher sequence than what we have. */
async function pollLive(): Promise<void> {
  if (eventsState.value !== "ok") return;
  livePollActive.value = true;
  try {
    const res = await listEvents({ filter: serverFilter.value });
    const cutoff = highestSequence.value;
    const fresh = res.events.filter((e) => e.sequence > cutoff);
    if (fresh.length) {
      /* Server returns newest-first; preserve that by putting fresh up top. */
      events.value = [...fresh, ...events.value];
      liveNewCount.value = fresh.length;
    } else {
      liveNewCount.value = 0;
    }
  } catch {
    /* Swallow — next tick will retry. We never want the live loop to
       blow up the page. */
  } finally {
    livePollActive.value = false;
  }
}

/* Keyboard shortcuts. ↑/↓ step through the visible event list while the
   detail rail is open. j/k mirror them (vim-style) for power users.
   The listener short-circuits when focus is in an input/textarea so it
   doesn't fight the search box. */
function onKeyDown(e: KeyboardEvent): void {
  const target = e.target as HTMLElement | null;
  const tag = target?.tagName ?? "";
  if (tag === "INPUT" || tag === "TEXTAREA" || target?.isContentEditable) return;
  if (!detailOpen.value || tab.value !== "events") return;
  if (e.key === "ArrowDown" || e.key === "j") { e.preventDefault(); stepEvent(+1); }
  else if (e.key === "ArrowUp" || e.key === "k") { e.preventDefault(); stepEvent(-1); }
}

function stepEvent(direction: number): void {
  const list = filteredEvents.value;
  if (list.length === 0) return;
  const idx = list.findIndex((e) => e.eventId === selectedEventId.value);
  const next = idx === -1
    ? (direction > 0 ? 0 : list.length - 1)
    : Math.max(0, Math.min(list.length - 1, idx + direction));
  openEvent(list[next].eventId);
}

onMounted(() => {
  loadStories({ reset: true });
  loadEvents({ reset: true });
  liveTimer = window.setInterval(pollLive, LIVE_POLL_MS);
  tickHandle = window.setInterval(() => { tickNow.value = new Date(); }, 30_000);
  document.addEventListener("keydown", onKeyDown);
});

onBeforeUnmount(() => {
  storiesAbort?.abort();
  eventsAbort?.abort();
  if (liveTimer !== null) window.clearInterval(liveTimer);
  if (tickHandle !== null) window.clearInterval(tickHandle);
  document.removeEventListener("keydown", onKeyDown);
});

/* ---- Client-side filtering ------------------------------------------ */

const needle = computed<string>(() => search.value.trim().toLowerCase());

function eventMatches(e: EventRecord, n: string): boolean {
  if (sourceFacet.value && e.source !== sourceFacet.value) return false;
  if (entityFacet.value && e.entity !== entityFacet.value) return false;
  if (interestingOnly.value && e.tags.length === 0) return false;
  if (!n) return true;
  return (
    e.kind.toLowerCase().includes(n) ||
    e.entity.toLowerCase().includes(n) ||
    e.source.toLowerCase().includes(n) ||
    e.eventId.toLowerCase().includes(n)
  );
}

function storyMatches(s: Story, n: string): boolean {
  if (sourceFacet.value && s.source !== sourceFacet.value) return false;
  if (entityFacet.value && !s.entityIds.includes(entityFacet.value)) return false;
  if (interestingOnly.value && s.tags.length === 0) return false;
  if (!n) return true;
  if (s.title.toLowerCase().includes(n)) return true;
  if (s.source.toLowerCase().includes(n)) return true;
  return s.entityIds.some((id) => id.toLowerCase().includes(n));
}

const filteredStories = computed<Story[]>(() =>
  stories.value.filter((s) => storyMatches(s, needle.value)),
);
const filteredEvents = computed<EventRecord[]>(() =>
  events.value.filter((e) => eventMatches(e, needle.value)),
);

/** True when any non-default filter is active (used to disable "Load more"
    since pagination would mix unfiltered server results with filtered
    state, which is confusing). */
const hasClientFilter = computed<boolean>(() =>
  !!needle.value || !!sourceFacet.value || !!entityFacet.value || interestingOnly.value,
);

/* ---- Empty-state copy ------------------------------------------------ */

const noStoriesHint = computed(() => {
  if (hasClientFilter.value && stories.value.length > 0) {
    return { title: "No matches", description: "Nothing in the current window matches. Try clearing filters or widening the time window." };
  }
  return { title: "Nothing yet", description: "No stories in this window. Try widening it." };
});
const noEventsHint = computed(() => {
  if (hasClientFilter.value && events.value.length > 0) {
    return { title: "No matches", description: "Nothing in the current window matches. Try clearing filters or widening the time window." };
  }
  return { title: "No events", description: "Nothing in this window. Try widening it." };
});

/* ---- Row / story helpers --------------------------------------------- */

function storyPresentation(s: Story): {
  icon: "sparkle" | "alert" | "activity"; intent: "automation" | "warn" | "info";
} {
  const cat = s.tags[0]?.category ?? "";
  if (cat === "failure" || cat === "security") return { icon: "alert",   intent: "warn"       };
  if (cat === "causation")                     return { icon: "sparkle", intent: "automation" };
  return { icon: "activity", intent: "info" };
}

/** Inner-event resolution: given a story, return whichever of its
    constituent events are present in the currently-loaded events list.
    Limited by what the events tab has loaded — beyond that we'd need a
    separate fetch per id. */
function innerEventsFor(s: Story): EventRecord[] {
  const wanted = new Set(s.innerEventIds);
  const found: EventRecord[] = [];
  for (const e of events.value) {
    if (wanted.has(e.eventId)) found.push(e);
  }
  /* Preserve the order from innerEventIds (chronological per proto). */
  found.sort((a, b) =>
    s.innerEventIds.indexOf(a.eventId) - s.innerEventIds.indexOf(b.eventId),
  );
  return found;
}

function toggleStory(s: Story): void {
  /* Single-event stories are functionally aliases of their one event;
     expanding them would just show one nested row. Skip the expansion
     and open the event detail rail directly, which is the screen the
     user actually wants. */
  if (s.innerEventIds.length === 1) {
    openEvent(s.innerEventIds[0]);
    return;
  }
  const next = new Set(expandedStories.value);
  if (next.has(s.id)) next.delete(s.id);
  else next.add(s.id);
  expandedStories.value = next;
}

function openEvent(id: string): void {
  selectedEventId.value = id;
  detailOpen.value = true;
}

function onSheetUpdate(open: boolean): void {
  detailOpen.value = open;
  if (!open) selectedEventId.value = "";
}

/* ---- Facet pin / clear ----------------------------------------------- */

function pinKind(v: string)   { kindFacet.value = v; }
function pinSource(v: string) { sourceFacet.value = v; }
function pinEntity(v: string) { entityFacet.value = v; }
function clearFacets(): void {
  kindFacet.value = "";
  sourceFacet.value = "";
  entityFacet.value = "";
  interestingOnly.value = false;
  search.value = "";
}

</script>

<template>
  <div class="page">
    <header class="page__head">
      <div class="page__heading">
        <SyText as="h1" variant="display">Activity</SyText>
        <!-- Live indicator: pulses while a poll is in flight; ticks the count
             when fresh rows arrive. -->
        <span class="page__live" :class="liveNewCount > 0 && 'page__live--fresh'">
          <SyDot :intent="liveNewCount > 0 ? 'good' : 'info'" :pulse="livePollActive ? 'fast' : 'slow'" />
          <SyText variant="caption" tone="subtle">
            Live<template v-if="liveNewCount > 0"> · +{{ liveNewCount }}</template>
          </SyText>
        </span>
      </div>
      <SyText variant="body" tone="subtle">
        Everything that happened. Coalesced into stories, or raw, with filters.
      </SyText>
    </header>

    <SyTabs
      :model-value="tab"
      :tabs="[
        { id: 'stories', label: 'Stories' },
        { id: 'events',  label: 'All events' },
      ]"
      @update:model-value="(v: string) => (tab = v as TabId)"
    />

    <div class="page__filters">
      <SySearchInput
        v-model="search"
        placeholder="Filter by entity, kind, source, or id…"
        class="page__search"
      />
      <SySegmented
        v-model="windowId"
        :options="WINDOWS.map((w) => ({ id: w.id, label: w.label }))"
        aria-label="Time window"
      />
      <label class="page__toggle">
        <SySwitch v-model="interestingOnly" />
        <SyText variant="caption">Interesting only</SyText>
      </label>
    </div>

    <!-- Active facet chips (kind / source / entity). Rendered only when at
         least one facet is set so the row collapses cleanly when empty. -->
    <div v-if="kindFacet || sourceFacet || entityFacet" class="page__chips">
      <SyFilterChip
        v-if="kindFacet"
        field="kind"
        :label="kindFacet"
        @remove="kindFacet = ''"
      />
      <SyFilterChip
        v-if="sourceFacet"
        field="source"
        :label="sourceFacet"
        @remove="sourceFacet = ''"
      />
      <SyFilterChip
        v-if="entityFacet"
        field="entity"
        :label="entityFacet"
        @remove="entityFacet = ''"
      />
      <SyButton intent="ghost" size="sm" @click="clearFacets">Clear all</SyButton>
    </div>

    <!-- Stories tab -->
    <template v-if="tab === 'stories'">
      <SySurface v-if="storiesState === 'loading'" padding="none">
        <SyEmptyState loading title="Loading stories…" />
      </SySurface>

      <SySurface v-else-if="storiesState === 'error'" padding="none">
        <SyEmptyState
          intent="bad"
          title="Couldn't load stories"
          :description="storiesError"
        >
          <template #icon><SyIcon name="close" :size="28" /></template>
          <template #actions>
            <SyButton intent="secondary" @click="loadStories({ reset: true })">Retry</SyButton>
          </template>
        </SyEmptyState>
      </SySurface>

      <SySurface v-else-if="filteredStories.length === 0" padding="none">
        <SyEmptyState :title="noStoriesHint.title" :description="noStoriesHint.description">
          <template #icon><SyIcon name="sparkle" :size="28" /></template>
        </SyEmptyState>
      </SySurface>

      <SySurface v-else padding="none" class="page__list">
        <SyStoryRow
          v-for="s in filteredStories"
          :key="s.id"
          interactive
          :expanded="expandedStories.has(s.id)"
          :icon="storyPresentation(s).icon"
          :intent="storyPresentation(s).intent"
          :title="s.title || 'Story'"
          :meta="s.entityIds.length ? s.entityIds.slice(0, 3).join(' · ') : s.source"
          :count="s.innerEventIds.length > 1 ? s.innerEventIds.length : 0"
          :timestamp="formatEventTimestamp(s.occurredAt, tickNow)"
          @toggle="toggleStory(s)"
        >
          <template #events>
            <SyEventRow
              v-for="ev in innerEventsFor(s)"
              :key="ev.eventId"
              interactive
              v-bind="presentEvent(ev, tickNow)"
              @click="openEvent(ev.eventId)"
            />
            <div
              v-if="innerEventsFor(s).length < s.innerEventIds.length"
              class="page__outsideHint"
            >
              <SyText variant="caption" tone="subtle">
                {{ s.innerEventIds.length - innerEventsFor(s).length }} event(s) outside the current event window. Switch to All events or widen the window to see them.
              </SyText>
            </div>
          </template>
        </SyStoryRow>
        <div v-if="storiesCursor && !hasClientFilter" class="page__more">
          <SyButton intent="secondary" @click="loadStories({ reset: false })">
            Load more
          </SyButton>
        </div>
      </SySurface>
    </template>

    <!-- All events tab -->
    <template v-else>
      <SySurface v-if="eventsState === 'loading'" padding="none">
        <SyEmptyState loading title="Loading events…" />
      </SySurface>

      <SySurface v-else-if="eventsState === 'error'" padding="none">
        <SyEmptyState
          intent="bad"
          title="Couldn't load events"
          :description="eventsError"
        >
          <template #icon><SyIcon name="close" :size="28" /></template>
          <template #actions>
            <SyButton intent="secondary" @click="loadEvents({ reset: true })">Retry</SyButton>
          </template>
        </SyEmptyState>
      </SySurface>

      <SySurface v-else-if="filteredEvents.length === 0" padding="none">
        <SyEmptyState :title="noEventsHint.title" :description="noEventsHint.description">
          <template #icon><SyIcon name="activity" :size="28" /></template>
        </SyEmptyState>
      </SySurface>

      <SySurface v-else padding="none" class="page__list">
        <SyEventRow
          v-for="e in filteredEvents"
          :key="e.eventId"
          interactive
          :selected="detailOpen && selectedEventId === e.eventId"
          v-bind="presentEvent(e, tickNow)"
          @click="openEvent(e.eventId)"
        />
        <div v-if="eventsCursor && !hasClientFilter" class="page__more">
          <SyButton intent="secondary" @click="loadEvents({ reset: false })">
            Load more
          </SyButton>
        </div>
      </SySurface>
    </template>

    <!-- Event detail sheet: opens when an event row is clicked. The
         detail component handles its own data fetch + state so we
         don't need a watcher here. Pinning a facet from inside the
         sheet closes the sheet so the user immediately sees the
         filtered list. -->
    <SySheet
      :modelValue="detailOpen"
      side="right"
      size="md"
      title="Event"
      @update:modelValue="onSheetUpdate"
    >
      <EventDetailContent
        :event-id="selectedEventId"
        @pin-kind="(v) => { pinKind(v); onSheetUpdate(false); }"
        @pin-source="(v) => { pinSource(v); onSheetUpdate(false); }"
        @pin-entity="(v) => { pinEntity(v); onSheetUpdate(false); }"
      />
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
.page__heading {
  display: flex;
  align-items: baseline;
  gap: var(--sy-space-3);
}
.page__live {
  display: inline-flex;
  align-items: center;
  gap: var(--sy-space-2);
}
.page__live--fresh :deep(.sy-dot) {
  /* Briefly tint when fresh rows arrive — animation runs once via the
     transition on color when we change the dot's intent. */
  filter: drop-shadow(0 0 4px var(--sy-color-good));
}
.page__filters {
  display: flex;
  align-items: center;
  gap: var(--sy-space-3);
  flex-wrap: wrap;
}
.page__search {
  flex: 1;
  min-width: 240px;
}
.page__toggle {
  display: inline-flex;
  align-items: center;
  gap: var(--sy-space-2);
  cursor: pointer;
  user-select: none;
}
.page__chips {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--sy-space-2);
}
.page__list :deep(.sy-listrow + .sy-listrow) {
  border-top: 1px solid var(--sy-color-line-soft);
}
.page__more {
  display: flex;
  justify-content: center;
  padding: var(--sy-space-3);
}
.page__outsideHint {
  padding: var(--sy-space-2) var(--sy-space-3) var(--sy-space-3);
}
</style>
