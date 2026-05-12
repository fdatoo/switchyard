<!--
  SyTabs — segmented tab strip.

  Data-driven: pass a `tabs` array of `TabDef`s instead of slot children, so
  tab sets can come straight from RPCs / config / URL state without
  composition gymnastics. The component is uncontrolled-by-default — pass
  `v-model="active"` for the controlled pattern.

  The panel content is the consumer's responsibility. SyTabs is just the
  strip. Render panels conditionally on the active id:
    `<div v-if="active === 'stories'">…</div>`

  ARIA: outer `role="tablist"`, each tab is a `role="tab"` button with
  `aria-selected`. The consumer's panel container should set
  `role="tabpanel"` and `aria-labelledby="<tab-id>"` to complete the pattern.

  Sliding indicator: a single absolutely-positioned pill behind the tabs
  provides the active-tab background. When the active id changes, the
  indicator transitions its `transform` and `width` to the new tab's
  position — the iconic Linear/Vercel/Material pattern. The transition uses
  the language's spring easing token so each language gets the right feel
  (overshoot on friendly/ambient, tighter on developer).
-->
<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import SyIcon from "@/lib/components/icon/SyIcon.vue";
import SyBadge from "@/lib/components/badge/SyBadge.vue";
import type { TabDef, TabsSize } from "./types";

const props = withDefaults(
  defineProps<{
    tabs: TabDef[];
    /** Currently-selected tab id. Use with `v-model`. */
    modelValue?: string;
    size?: TabsSize;
    /** Stretch tabs to fill the available width. */
    fill?: boolean;
  }>(),
  { size: "md", fill: false },
);

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const active = computed(() =>
  props.modelValue ?? props.tabs.find((t) => !t.disabled)?.id ?? "",
);

function select(tab: TabDef): void {
  if (tab.disabled) return;
  emit("update:modelValue", tab.id);
}

/* Sliding indicator. We measure the active tab's position relative to the
   tablist container and apply `transform: translateX(...) + width` to a
   single pseudo-tab beneath the buttons. */
const containerRef = ref<HTMLElement | null>(null);
const indicatorStyle = ref<Record<string, string>>({ opacity: "0" });

function updateIndicator(): void {
  const container = containerRef.value;
  if (!container) return;
  const tab = container.querySelector<HTMLElement>(`[data-tab-id="${active.value}"]`);
  if (!tab) {
    indicatorStyle.value = { ...indicatorStyle.value, opacity: "0" };
    return;
  }
  const cRect = container.getBoundingClientRect();
  const tRect = tab.getBoundingClientRect();
  indicatorStyle.value = {
    width: `${tRect.width}px`,
    transform: `translateX(${tRect.left - cRect.left}px)`,
    opacity: "1",
  };
}

/* The indicator depends on layout. Update on mount (after paint), whenever
   the active tab or the tabs prop changes, and whenever the container
   itself resizes (window resize, fill-mode reflow). */
let resizeObserver: ResizeObserver | null = null;

onMounted(() => {
  nextTick(updateIndicator);
  if (containerRef.value && typeof ResizeObserver !== "undefined") {
    resizeObserver = new ResizeObserver(() => updateIndicator());
    resizeObserver.observe(containerRef.value);
  }
});

onUnmounted(() => {
  resizeObserver?.disconnect();
});

watch(
  () => [active.value, props.tabs, props.size, props.fill],
  () => nextTick(updateIndicator),
);

const classes = computed(() => [
  "sy-tabs",
  `sy-tabs--${props.size}`,
  props.fill && "sy-tabs--fill",
]);
</script>

<template>
  <div ref="containerRef" :class="classes" role="tablist">
    <span class="sy-tabs__indicator" :style="indicatorStyle" aria-hidden="true" />

    <button
      v-for="tab in tabs"
      :id="tab.id"
      :key="tab.id"
      :data-tab-id="tab.id"
      type="button"
      role="tab"
      class="sy-tab"
      :aria-selected="active === tab.id"
      :tabindex="active === tab.id ? 0 : -1"
      :disabled="tab.disabled"
      @click="select(tab)"
    >
      <SyIcon v-if="tab.icon" :name="tab.icon" :size="size === 'sm' ? 14 : size === 'lg' ? 18 : 16" />
      <span class="sy-tab__label">{{ tab.label }}</span>
      <SyBadge
        v-if="tab.badge"
        size="sm"
        :intent="tab.badge.intent ?? 'neutral'"
        appearance="soft"
        class="sy-tab__badge"
      >
        {{ tab.badge.count }}
      </SyBadge>
    </button>
  </div>
</template>

<style scoped>
.sy-tabs {
  position: relative;
  display: inline-flex;
  gap: 2px;
  padding: 3px;
  /* surface-3 instead of surface-2 — gives the surface-1 indicator clear
     contrast in light languages where the surface tiers step gently. */
  background: var(--sy-color-surface-3);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-pill);
  max-width: 100%;
  /* `clip` instead of `auto`: the spring-overshoot on the indicator briefly
     extends past the right edge, and `auto` would interpret that as "needs
     scrolling" and flash a scrollbar mid-animation. `clip` invisibly crops
     the overshoot. Tabs aren't meant to scroll — consumers with too-many
     tabs should switch to a different pattern. */
  overflow: clip;
}

.sy-tabs--fill {
  display: flex;
  width: 100%;
}
.sy-tabs--fill .sy-tab { flex: 1; justify-content: center; }

/* Sliding indicator: paints the active tab's background. Lives behind the
   buttons (z-index 0) while buttons stack at z-index 1, so labels remain
   tappable and selectable above the moving pill. */
.sy-tabs__indicator {
  position: absolute;
  top: 3px;
  bottom: 3px;
  left: 0;
  background: var(--sy-color-surface-1);
  border-radius: var(--sy-radius-pill);
  box-shadow: var(--sy-shadow);
  pointer-events: none;
  z-index: 0;
  /* Spring easing gives a satisfying overshoot on combo switches. The
     280ms duration matches SySurface's press transition for consistency. */
  transition: transform 320ms var(--sy-motion-spring),
              width 320ms var(--sy-motion-spring),
              opacity var(--sy-motion-fast);
}

.sy-tab {
  position: relative;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  gap: var(--sy-space-2);
  background: transparent;
  border: 0;
  border-radius: var(--sy-radius-pill);
  color: var(--sy-color-fg-3);
  font-family: var(--sy-font-body);
  font-weight: 500;
  letter-spacing: -0.005em;
  cursor: pointer;
  white-space: nowrap;
  /* Color transitions fast; the indicator handles the bigger motion. */
  transition: color var(--sy-motion-fast);
}

.sy-tabs--sm .sy-tab { padding: 3px 10px; font-size: 0.75rem; }
.sy-tabs--md .sy-tab { padding: 5px 14px; font-size: 0.8125rem; }
.sy-tabs--lg .sy-tab { padding: 7px 18px; font-size: 0.9375rem; }

.sy-tab:hover:not(:disabled):not([aria-selected="true"]) {
  color: var(--sy-color-fg);
}

.sy-tab[aria-selected="true"] {
  color: var(--sy-color-fg);
  font-weight: 600;
}

.sy-tab:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.sy-tab:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}

.sy-tab__label {
  min-width: 0;
}
.sy-tab__badge {
  margin-left: 2px;
}
</style>
