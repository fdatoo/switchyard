<!--
  SyEventRow — single Activity feed row.

  Renders one event from the event log: an icon + title + optional context +
  timestamp. The Activity surface uses this for the "All events" tab; the
  detail right rail uses it for the per-entity recent-events list.

  Composes tier 1+2: SyListRow for the row layout, SyIcon for the leading
  glyph, SyText for typography, SyDot for an inline cause/effect hint when
  a causation chain is provided.

  Intent maps to a color for the icon and the optional cause/effect dot:
    - `automation` (purple): event was triggered by an automation
    - `good`/`warn`/`bad`: state-change events with semantic meaning
    - `info`/`neutral`/`accent`: everything else

  Pre-formatted timestamp string is the consumer's responsibility (e.g.,
  "12:42" / "2 min ago" / "Mar 14"). Formatting is too locale/relative-
  time-rules dependent to bake into a primitive.
-->
<script setup lang="ts">
import { computed } from "vue";
import SyListRow from "@/lib/components/listrow/SyListRow.vue";
import SyIcon, { type IconName } from "@/lib/components/icon/SyIcon.vue";
import SyText from "@/lib/components/text/SyText.vue";
import SyBadge from "@/lib/components/badge/SyBadge.vue";

type Intent = "neutral" | "accent" | "good" | "warn" | "bad" | "info" | "automation";
type BadgeIntent = "neutral" | "accent" | "good" | "warn" | "bad" | "info" | "purple";

/** Per-row interestingness tag stripe. Up to ~2 are rendered; more get a
    "+N" overflow chip. Each tag carries its category-derived intent so
    failure/security visibly pop red even at small badge size. */
export interface EventRowTag {
  intent: BadgeIntent;
  label: string;
}

const props = withDefaults(
  defineProps<{
    /** Leading icon. Choose by event type (bulb / power / sparkle / plugin / etc.). */
    icon: IconName;
    /** Color the leading icon. */
    intent?: Intent;
    /** Primary event summary, e.g., "light.kitchen_pendant turned on". */
    title: string;
    /** Secondary line: source entity, automation name, etc. */
    meta?: string;
    /** Pre-formatted relative or absolute timestamp. */
    timestamp: string;
    /**
     * Cause-and-effect hint, rendered as "<from> → <to>" inline. Used for
     * automation-triggered events ("from sunset_lights") and chained
     * effects ("→ scene.movie_time").
     */
    cause?: string;
    /** Interestingness signals rendered as a thin badge stripe before the timestamp. */
    tags?: EventRowTag[];
    interactive?: boolean;
    /** Mark the row as the active selection (e.g., its detail rail is open). */
    selected?: boolean;
  }>(),
  { intent: "info", interactive: false, selected: false, tags: () => [] },
);

const visibleTags = computed(() => props.tags.slice(0, 2));
const overflowTagCount = computed(() => Math.max(0, props.tags.length - visibleTags.value.length));

/* Map intent to a CSS color (used to tint the leading icon). Purple is
   reserved for automation-triggered events per the vision spec's
   interestingness taxonomy. */
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
</script>

<template>
  <SyListRow
    density="compact"
    :interactive="interactive"
    :bordered="false"
    :selected="selected"
    class="sy-event"
  >
    <template #leading>
      <span class="sy-event__icon" :style="{ color: intentColor }">
        <SyIcon :name="icon" :size="16" />
      </span>
    </template>

    <div class="sy-event__title-row">
      <SyText as="span" variant="body" weight="medium" class="sy-event__title">
        {{ title }}
      </SyText>
      <SyText
        v-if="cause"
        as="span"
        variant="caption"
        tone="subtle"
        class="sy-event__cause"
      >
        {{ cause }}
      </SyText>
    </div>
    <SyText v-if="meta" variant="caption" tone="subtle">{{ meta }}</SyText>

    <template #trailing>
      <span v-if="tags.length" class="sy-event__tags">
        <SyBadge
          v-for="t in visibleTags"
          :key="t.label"
          :intent="t.intent"
          size="sm"
        >{{ t.label }}</SyBadge>
        <SyBadge v-if="overflowTagCount > 0" intent="neutral" size="sm">
          +{{ overflowTagCount }}
        </SyBadge>
      </span>
      <SyText variant="caption" tone="subtle" class="sy-event__time">
        {{ timestamp }}
      </SyText>
    </template>
  </SyListRow>
</template>

<style scoped>
/* Enforce a 2-line minimum height so feeds scan as a steady column. Single-
   line rows (no `meta`) get extra vertical breathing room rather than
   compressing — variable row heights kill scannability in dense lists. */
.sy-event :deep(.sy-listrow) {
  min-height: 52px;
}

.sy-event__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: var(--sy-radius-sm);
  /* Subtle tinted bg using currentColor + color-mix; doesn't need a new
     token because the icon's intent color is already on the element. */
  background: color-mix(in srgb, currentColor 10%, transparent);
}

.sy-event__title-row {
  display: inline-flex;
  align-items: baseline;
  gap: var(--sy-space-2);
  min-width: 0;
  flex-wrap: wrap;
}

.sy-event__title {
  /* Allow ellipsis on long entity names without pushing the cause off. */
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sy-event__cause {
  white-space: nowrap;
}

/* Numeric font for the timestamp gives a steady column when many rows stack. */
.sy-event__tags {
  display: inline-flex;
  gap: 4px;
  align-items: center;
  margin-right: var(--sy-space-2);
}

.sy-event__time {
  font-family: var(--sy-font-numeric);
  font-feature-settings: var(--sy-numeric-feature);
  font-size: 0.6875rem;
}
</style>
