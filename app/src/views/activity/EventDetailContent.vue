<!--
  EventDetailContent — the body of the event detail right rail.

  Renders one event's full record: rich title from payload, the
  interestingness tag list, identity fields (ids, sequence, timestamp),
  the parsed payload as pretty JSON, and the causation chain (ancestors)
  as a small inline event list.

  Loads its own data: takes an `eventId` prop and fetches via the
  ActivityService/EventDetail RPC. Owning the fetch here (instead of in
  ActivityView) keeps the parent focused on list-level concerns.
-->
<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import {
  SyText, SyBadge, SySurface, SyEmptyState, SyIcon, SyButton, SyEventRow,
} from "@/lib";
import { eventDetail, type EventRecord, type InterestingnessTag } from "@/data/activity";
import { presentEvent, formatEventTimestamp, tagBadgeIntent } from "@/data/event-display";

const props = defineProps<{
  /** Event id to fetch + render. Empty/undefined renders nothing. */
  eventId: string;
}>();

const emit = defineEmits<{
  /** Bubble up so the parent can pin a facet filter for this value. */
  "pin-kind":   [value: string];
  "pin-source": [value: string];
  "pin-entity": [value: string];
}>();

type LoadState = "loading" | "ok" | "error";

const detail = ref<{ event: EventRecord; chain: EventRecord[] } | null>(null);
const state = ref<LoadState>("loading");
const errorMessage = ref<string>("");
let abort: AbortController | null = null;

watch(() => props.eventId, (id) => {
  abort?.abort();
  detail.value = null;
  if (!id) return;
  abort = new AbortController();
  state.value = "loading";
  errorMessage.value = "";
  eventDetail(id, { signal: abort.signal })
    .then((res) => {
      detail.value = { event: res.event, chain: res.causationChain };
      state.value = "ok";
    })
    .catch((err: Error) => {
      if (err.name === "AbortError") return;
      state.value = "error";
      errorMessage.value = err.message;
    });
}, { immediate: true });

onUnmounted(() => abort?.abort());

const prettyPayload = computed<string>(() => {
  const raw = detail.value?.event.payloadJson ?? "";
  if (!raw) return "";
  try { return JSON.stringify(JSON.parse(raw), null, 2); }
  catch { return raw; }
});

/* Map a tag to a badge intent for the stripe colors. Re-exported from
   event-display so the row stripe and the detail list stay in lockstep. */
function tagIntent(t: InterestingnessTag): ReturnType<typeof tagBadgeIntent> {
  return tagBadgeIntent(t);
}

/* Copy-to-clipboard for the payload block. Toggling `copied` flips the
   button's icon + label for 1.5s so the user sees a confirmation
   without a separate toast. */
const copied = ref<boolean>(false);
async function copyPayload(): Promise<void> {
  const text = prettyPayload.value;
  if (!text) return;
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      /* Fallback for older browsers / file:// pages. */
      const ta = document.createElement("textarea");
      ta.value = text; ta.style.position = "fixed"; ta.style.opacity = "0";
      document.body.appendChild(ta); ta.select();
      document.execCommand("copy");
      document.body.removeChild(ta);
    }
    copied.value = true;
    setTimeout(() => { copied.value = false; }, 1500);
  } catch {
    /* Silently ignore — clipboard can be denied in untrusted contexts. */
  }
}
</script>

<template>
  <div v-if="!eventId" class="empty">
    <SyText variant="caption" tone="subtle">Select an event to see details.</SyText>
  </div>

  <SyEmptyState
    v-else-if="state === 'loading'"
    loading
    title="Loading event…"
  />

  <SyEmptyState
    v-else-if="state === 'error'"
    intent="bad"
    title="Couldn't load event"
    :description="errorMessage"
  >
    <template #icon><SyIcon name="close" :size="28" /></template>
  </SyEmptyState>

  <div v-else-if="detail" class="detail">
    <!-- Headline: rich title + relative timestamp -->
    <header class="detail__head">
      <SyText weight="medium" variant="title">
        {{ presentEvent(detail.event).title }}
      </SyText>
      <SyText variant="caption" tone="subtle">
        {{ formatEventTimestamp(detail.event.occurredAt) }} ·
        {{ detail.event.occurredAt.toLocaleString() }}
      </SyText>
    </header>

    <!-- Interestingness tags -->
    <div v-if="detail.event.tags.length" class="detail__tags">
      <SyBadge
        v-for="t in detail.event.tags"
        :key="`${t.category}/${t.name}`"
        :intent="tagIntent(t)"
        :title="t.explanation"
      >
        {{ t.category }} · {{ t.name }}
      </SyBadge>
    </div>

    <!-- Identity facts -->
    <dl class="detail__facts">
      <div class="detail__fact">
        <dt><SyText variant="caption" tone="subtle">Kind</SyText></dt>
        <dd>
          <button class="detail__pin" @click="emit('pin-kind', detail.event.kind)">
            <SyText weight="medium">{{ detail.event.kind }}</SyText>
            <SyIcon name="filter" :size="12" />
          </button>
        </dd>
      </div>
      <div v-if="detail.event.entity" class="detail__fact">
        <dt><SyText variant="caption" tone="subtle">Entity</SyText></dt>
        <dd>
          <button class="detail__pin" @click="emit('pin-entity', detail.event.entity)">
            <SyText weight="medium">{{ detail.event.entity }}</SyText>
            <SyIcon name="filter" :size="12" />
          </button>
        </dd>
      </div>
      <div v-if="detail.event.source" class="detail__fact">
        <dt><SyText variant="caption" tone="subtle">Source</SyText></dt>
        <dd>
          <button class="detail__pin" @click="emit('pin-source', detail.event.source)">
            <SyText weight="medium">{{ detail.event.source }}</SyText>
            <SyIcon name="filter" :size="12" />
          </button>
        </dd>
      </div>
      <div class="detail__fact">
        <dt><SyText variant="caption" tone="subtle">Sequence</SyText></dt>
        <dd><SyText variant="body">{{ detail.event.sequence }}</SyText></dd>
      </div>
      <div v-if="detail.event.correlationId" class="detail__fact">
        <dt><SyText variant="caption" tone="subtle">Correlation</SyText></dt>
        <dd><SyText variant="caption" class="detail__mono">{{ detail.event.correlationId }}</SyText></dd>
      </div>
      <div v-if="detail.event.causationId" class="detail__fact">
        <dt><SyText variant="caption" tone="subtle">Caused by</SyText></dt>
        <dd><SyText variant="caption" class="detail__mono">{{ detail.event.causationId }}</SyText></dd>
      </div>
    </dl>

    <!-- Causation chain -->
    <section v-if="detail.chain.length" class="detail__section">
      <SyText variant="overline" tone="subtle" weight="medium">Caused by</SyText>
      <SySurface padding="none">
        <SyEventRow
          v-for="ev in detail.chain"
          :key="ev.eventId"
          v-bind="presentEvent(ev)"
        />
      </SySurface>
    </section>

    <!-- Raw payload -->
    <section v-if="prettyPayload" class="detail__section">
      <div class="detail__sectionHead">
        <SyText variant="overline" tone="subtle" weight="medium">Payload</SyText>
        <button
          type="button"
          class="detail__copy"
          :aria-label="copied ? 'Payload copied' : 'Copy payload to clipboard'"
          @click="copyPayload"
        >
          <SyIcon :name="copied ? 'check' : 'filter'" :size="12" />
          <span>{{ copied ? "Copied" : "Copy" }}</span>
        </button>
      </div>
      <SySurface>
        <pre class="detail__pre">{{ prettyPayload }}</pre>
      </SySurface>
    </section>
  </div>
</template>

<style scoped>
.empty {
  padding: var(--sy-space-4);
}
.detail {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-4);
}
.detail__head {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-1);
}
.detail__tags {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sy-space-2);
}
.detail__facts {
  display: grid;
  grid-template-columns: 110px minmax(0, 1fr);
  gap: var(--sy-space-2) var(--sy-space-3);
  margin: 0;
}
.detail__fact {
  display: contents; /* So the parent grid lays out dt/dd in two columns. */
}
.detail__fact dt, .detail__fact dd { margin: 0; min-width: 0; }
/* Pinnable identity values: hover lifts; click emits a `pin-*` event so
   the parent can add it as a removable filter chip. */
.detail__pin {
  display: inline-flex;
  align-items: center;
  gap: var(--sy-space-1);
  padding: 2px 6px;
  margin: -2px -6px;
  background: transparent;
  border: 0;
  border-radius: var(--sy-radius);
  cursor: pointer;
  color: inherit;
  font: inherit;
  transition: background var(--sy-motion-fast);
}
.detail__pin:hover {
  background: var(--sy-color-surface-2);
}
.detail__pin:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 1px;
}
.detail__mono {
  font-family: var(--sy-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  word-break: break-all;
}
.detail__section {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-2);
}
.detail__sectionHead {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.detail__copy {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  background: transparent;
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-pill);
  cursor: pointer;
  font: inherit;
  font-size: 0.7rem;
  color: var(--sy-color-fg-2);
  transition: background var(--sy-motion-fast), color var(--sy-motion-fast);
}
.detail__copy:hover {
  background: var(--sy-color-surface-2);
  color: var(--sy-color-fg);
}
.detail__pre {
  margin: 0;
  font-family: var(--sy-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 0.78rem;
  line-height: 1.5;
  color: var(--sy-color-fg);
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
