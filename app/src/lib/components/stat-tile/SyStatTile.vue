<!--
  SyStatTile — a single at-a-glance metric for dashboards.

  Composed of three slots: icon (leading), the numeric value, and a
  trailing detail line. Made tappable when `to` is set so the tile acts
  as a quick-link into the corresponding detail page.

  Intent tints the icon (matching SyEventRow/SyDot semantics) and gets
  a faint background pill — same `color-mix` trick the event row uses
  so we don't add new tokens just for soft variants.
-->
<script setup lang="ts">
import { computed } from "vue";
import { useRouter } from "vue-router";
import SyIcon, { type IconName } from "@/lib/components/icon/SyIcon.vue";
import SyText from "@/lib/components/text/SyText.vue";
import SySpinner from "@/lib/components/spinner/SySpinner.vue";

type Intent = "neutral" | "accent" | "good" | "warn" | "bad" | "info" | "automation";

const props = withDefaults(
  defineProps<{
    /** Icon name from SyIcon's catalog. */
    icon: IconName;
    /** Color the icon. */
    intent?: Intent;
    /** Short uppercase eyebrow. */
    label: string;
    /** Big number / string. Pass `undefined` while loading to show a spinner. */
    value?: string | number;
    /** Secondary line — supporting detail like "of 16" or "active". */
    detail?: string;
    /** When set, the tile becomes a router link to this path. */
    to?: string;
    /** Loading shows a spinner instead of the value. */
    loading?: boolean;
  }>(),
  { intent: "info", loading: false },
);

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

const router = useRouter();
const isLink = computed(() => !!props.to);
function onClick(): void {
  if (props.to) router.push(props.to);
}
</script>

<template>
  <component
    :is="isLink ? 'button' : 'div'"
    :type="isLink ? 'button' : undefined"
    class="sy-stat"
    :class="[isLink && 'sy-stat--interactive']"
    @click="isLink ? onClick() : null"
  >
    <span class="sy-stat__icon" :style="{ color: intentColor }">
      <SyIcon :name="icon" :size="20" />
    </span>
    <div class="sy-stat__body">
      <SyText variant="overline" tone="subtle" weight="medium" class="sy-stat__label">
        {{ label }}
      </SyText>
      <div class="sy-stat__valueRow">
        <SySpinner v-if="loading" :size="20" />
        <SyText v-else as="span" variant="display" weight="semibold" class="sy-stat__value">
          {{ value }}
        </SyText>
        <SyText v-if="detail" variant="caption" tone="subtle" class="sy-stat__detail">
          {{ detail }}
        </SyText>
      </div>
    </div>
  </component>
</template>

<style scoped>
.sy-stat {
  display: flex;
  align-items: flex-start;
  gap: var(--sy-space-3);
  padding: var(--sy-space-4);
  background: var(--sy-color-surface-1);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-lg);
  color: inherit;
  font: inherit;
  text-align: left;
  width: 100%;
  cursor: default;
  transition: border-color var(--sy-motion-fast),
              background var(--sy-motion-fast),
              transform var(--sy-motion-fast);
}

.sy-stat--interactive {
  cursor: pointer;
}
.sy-stat--interactive:hover {
  border-color: var(--sy-color-fg-5);
  background: var(--sy-color-surface-2);
}
.sy-stat--interactive:active {
  transform: scale(0.997);
}
.sy-stat--interactive:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}

.sy-stat__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: var(--sy-radius);
  background: color-mix(in srgb, currentColor 12%, transparent);
  flex-shrink: 0;
}

.sy-stat__body {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-1);
  min-width: 0;
  flex: 1;
}

.sy-stat__label {
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.sy-stat__valueRow {
  display: flex;
  align-items: baseline;
  gap: var(--sy-space-2);
  min-width: 0;
}

.sy-stat__value {
  /* Display tokens are heavy; tile values benefit from numeric features. */
  font-feature-settings: var(--sy-numeric-feature, "tnum");
}

.sy-stat__detail {
  white-space: nowrap;
}
</style>
