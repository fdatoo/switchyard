<!--
  SyStoryRow — coalesced Activity story.

  A Story is a small narrative composed from multiple related events:
  "Sunset routine ran → 4 lights on" instead of four separate "light on"
  rows. Per the vision spec's interestingness pipeline, the daemon
  coalesces correlated events into Stories; this component renders the
  summary.

  Composition: a comfortable-density row with a larger tinted icon tile, a
  title + meta stack, and a trailing column with the event count and a
  timestamp range. Optional `expanded` + `events` slot renders constituent
  rows underneath — typically populated with SyEventRow children.

  The expand affordance (chevron) is the consumer's responsibility to
  trigger via the `toggle` event. We don't manage open state internally so
  consumers can persist it, animate it differently, or coordinate across
  multiple stories (e.g., "expand all").
-->
<script setup lang="ts">
import { computed } from "vue";
import SyListRow from "@/lib/components/listrow/SyListRow.vue";
import SyIcon, { type IconName } from "@/lib/components/icon/SyIcon.vue";
import SyText from "@/lib/components/text/SyText.vue";
import SyBadge from "@/lib/components/badge/SyBadge.vue";

type Intent = "neutral" | "accent" | "good" | "warn" | "bad" | "info" | "automation";

const props = withDefaults(
  defineProps<{
    icon: IconName;
    intent?: Intent;
    title: string;
    /** Short summary of involved entities or context. */
    meta?: string;
    /** Number of constituent events. Rendered as a count badge. */
    count: number;
    /** Timestamp or range (e.g., "12:42 — 12:47"). */
    timestamp: string;
    /** Whether the constituent events slot is rendered. */
    expanded?: boolean;
    /** Whether the row is clickable (also enables the chevron affordance). */
    interactive?: boolean;
  }>(),
  { intent: "automation", expanded: false, interactive: false },
);

const emit = defineEmits<{
  toggle: [];
}>();

const intentColor = computed(() => {
  switch (props.intent) {
    case "accent":     return "var(--sy-color-accent)";
    case "good":       return "var(--sy-color-good)";
    case "warn":       return "var(--sy-color-warn)";
    case "bad":        return "var(--sy-color-bad)";
    case "info":       return "var(--sy-color-info)";
    case "automation": return "var(--sy-color-purple)";
    default:           return "var(--sy-color-fg-3)";
  }
});

/* Badge intent maps from row intent, with `neutral`/`info` falling through
   to a quieter chip color than the bold purple/red of automation/bad. */
const badgeIntent = computed(() => {
  switch (props.intent) {
    case "automation": return "purple";
    case "neutral":    return "neutral";
    default:           return props.intent;
  }
});
</script>

<template>
  <div class="sy-story">
    <SyListRow
      density="comfortable"
      :interactive="interactive"
      :bordered="false"
      class="sy-story__head"
      @click="interactive ? emit('toggle') : null"
    >
      <template #leading>
        <span class="sy-story__icon" :style="{ color: intentColor }">
          <SyIcon :name="icon" :size="20" />
        </span>
      </template>

      <SyText variant="subtitle" weight="semibold" class="sy-story__title">
        {{ title }}
      </SyText>
      <SyText v-if="meta" variant="caption" tone="subtle">{{ meta }}</SyText>

      <template #trailing>
        <!-- Hide the count badge when count is 0 — single-event stories
             pass 0 so the badge collapses (the row implies one event). -->
        <SyBadge v-if="count > 0" :intent="badgeIntent" appearance="soft" size="sm">
          {{ count }} {{ count === 1 ? "event" : "events" }}
        </SyBadge>
        <SyText variant="caption" tone="subtle" class="sy-story__time">
          {{ timestamp }}
        </SyText>
        <span
          v-if="interactive"
          class="sy-story__chev"
          :class="[expanded && 'sy-story__chev--open']"
          aria-hidden="true"
        >
          <SyIcon name="chevron-down" :size="14" />
        </span>
      </template>
    </SyListRow>

    <div v-if="expanded && $slots.events" class="sy-story__events">
      <slot name="events" />
    </div>
  </div>
</template>

<style scoped>
.sy-story {
  display: flex;
  flex-direction: column;
  /* No own background. The wrapping surface (SySurface, list group, etc.)
     provides the one shared bg layer for the feed; stories separate
     themselves visually via a hairline border between siblings (set by
     the consumer). Adding a layer here would brighten each story tile
     against a translucent ambient surface, creating "story tiles with
     darker gaps between" — exactly what we don't want. */
}

/* SyListRow's default `surface-1` would add its own layer on top of the
   wrapping surface. Strip it so the entire story (head + events) sits as
   one uniform translucent layer. Hover/active states still apply because
   their selectors include the pseudo-classes. */
.sy-story :deep(.sy-listrow:not(:hover):not(:active)) {
  background: transparent;
}

.sy-story__head :deep(.sy-listrow) {
  min-height: 60px;
}

.sy-story__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: var(--sy-radius);
  /* Tinted bg derived from the icon's intent color via currentColor +
     color-mix. Same pattern as SyEventRow's smaller variant. */
  background: color-mix(in srgb, currentColor 12%, transparent);
}

.sy-story__title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sy-story__time {
  font-family: var(--sy-font-numeric);
  font-feature-settings: var(--sy-numeric-feature);
  font-size: 0.6875rem;
}

.sy-story__chev {
  display: inline-flex;
  align-items: center;
  color: var(--sy-color-fg-4);
  transition: transform var(--sy-motion);
}
.sy-story__chev--open {
  transform: rotate(180deg);
}

/* Events panel: transparent so the wrapper's bg shows through uniformly.
   The thread-rail is drawn as a pseudo-element over the panel. */
.sy-story__events {
  position: relative;
  padding-left: calc(var(--sy-space-4) + 18px + var(--sy-space-3));
  padding-bottom: var(--sy-space-2);
}
.sy-story__events::before {
  content: "";
  position: absolute;
  left: calc(var(--sy-space-4) + 17px);
  top: 0;
  bottom: var(--sy-space-2);
  width: 2px;
  background: color-mix(in srgb, var(--sy-color-line) 70%, transparent);
  border-radius: 2px;
}


/* Ambient: the default `currentColor 12%` icon-tile bg disappears against
   ambient's translucent surface. The rail also needs a bumped explicit
   color so it stays visible against the glass. */
[data-language="ambient"] .sy-story__icon,
.sy-story[data-language="ambient"] .sy-story__icon {
  background: color-mix(in srgb, currentColor 24%, transparent);
}
[data-language="ambient"] .sy-story__events::before,
.sy-story[data-language="ambient"] .sy-story__events::before {
  background: rgba(255, 255, 255, 0.22);
}
</style>
