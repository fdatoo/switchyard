<!--
  SyTooltip — hover/focus popover.

  Wraps a trigger element (default slot) and shows a tooltip on hover or
  keyboard focus after a brief delay. The tooltip is teleported to `<body>`
  so it isn't clipped by ancestor `overflow: hidden` containers or trapped
  in a low-z-index stacking context.

  Positioning is hand-rolled: we measure the anchor's bounding rect after
  the tooltip mounts, place it on the requested side, and flip to the
  opposite side if it would overflow the viewport. No full positioning lib
  (Floating UI etc.) — the four-side + viewport-flip subset is enough for
  v1.

  Triggers: hover (mouseenter/leave) + keyboard focus (focusin/focusout).
  Both schedule a show after `delay` (default 200ms) and hide instantly.

  Note: because the tooltip is teleported to body, it inherits the
  document-level [data-language] / [data-mode], not the anchor's. In the
  lab, that means tooltips from a per-cell-themed cell render in the
  global theme. That's a fair tradeoff — fixing it would require a
  per-anchor theme attribute on the teleported element, which is over-
  engineering for v1.
-->
<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref } from "vue";
import { applyAnchorTheme } from "@/lib/theme/inherit-theme";

type Side = "top" | "bottom" | "left" | "right";

const props = withDefaults(
  defineProps<{
    /** Tooltip text. Use the `content` slot for rich content. */
    content?: string;
    /** Preferred side. Flips to the opposite if it would overflow the viewport. */
    side?: Side;
    /** Show delay in ms. Hide is always instant. */
    delay?: number;
    disabled?: boolean;
  }>(),
  { side: "top", delay: 200, disabled: false },
);

const anchorRef = ref<HTMLElement | null>(null);
const popperRef = ref<HTMLElement | null>(null);
const visible = ref(false);
const popperStyle = ref<Record<string, string>>({});
const resolvedSide = ref<Side>(props.side);

let showTimer: ReturnType<typeof setTimeout> | null = null;

function schedule(): void {
  if (props.disabled) return;
  if (showTimer) clearTimeout(showTimer);
  showTimer = setTimeout(() => {
    visible.value = true;
    nextTick(() => {
      if (popperRef.value) applyAnchorTheme(popperRef.value, anchorRef.value);
      updatePosition();
    });
  }, props.delay);
}

function cancel(): void {
  if (showTimer) {
    clearTimeout(showTimer);
    showTimer = null;
  }
  visible.value = false;
}

/**
 * Compute the tooltip's fixed-position coordinates. Tries the preferred
 * side first; if the tooltip would overflow the viewport on that side,
 * flips to the opposite. Centering on the perpendicular axis is clamped
 * to keep the tooltip on-screen.
 */
function updatePosition(): void {
  const anchor = anchorRef.value;
  const popper = popperRef.value;
  if (!anchor || !popper) return;

  const a = anchor.getBoundingClientRect();
  const p = popper.getBoundingClientRect();
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const gap = 6;
  const pad = 8; /* viewport edge padding for clamps */

  let side: Side = props.side;
  /* Flip if the preferred side would overflow. */
  if (side === "top" && a.top - p.height - gap < pad) side = "bottom";
  else if (side === "bottom" && a.bottom + p.height + gap > vh - pad) side = "top";
  else if (side === "left" && a.left - p.width - gap < pad) side = "right";
  else if (side === "right" && a.right + p.width + gap > vw - pad) side = "left";

  let top = 0;
  let left = 0;
  if (side === "top" || side === "bottom") {
    left = a.left + a.width / 2 - p.width / 2;
    left = Math.max(pad, Math.min(left, vw - p.width - pad));
    top = side === "top" ? a.top - p.height - gap : a.bottom + gap;
  } else {
    top = a.top + a.height / 2 - p.height / 2;
    top = Math.max(pad, Math.min(top, vh - p.height - pad));
    left = side === "left" ? a.left - p.width - gap : a.right + gap;
  }

  resolvedSide.value = side;
  popperStyle.value = {
    top: `${top}px`,
    left: `${left}px`,
  };
}

onUnmounted(() => {
  if (showTimer) clearTimeout(showTimer);
});

const tooltipClass = computed(() => [
  "sy-tooltip",
  `sy-tooltip--${resolvedSide.value}`,
]);
</script>

<template>
  <span
    ref="anchorRef"
    class="sy-tooltip__anchor"
    @mouseenter="schedule"
    @mouseleave="cancel"
    @focusin="schedule"
    @focusout="cancel"
  >
    <slot />
  </span>
  <Teleport to="body">
    <Transition name="sy-tooltip">
      <div
        v-if="visible"
        ref="popperRef"
        :class="tooltipClass"
        :style="popperStyle"
        role="tooltip"
      >
        <slot name="content">{{ content }}</slot>
      </div>
    </Transition>
  </Teleport>
</template>

<style>
/* Unscoped: the teleport target (body) is outside this component's scope,
   so scoped styles wouldn't reach the popper. Class is namespaced enough
   (`sy-tooltip`) that this isn't a leak risk. */
.sy-tooltip {
  position: fixed;
  z-index: 100;
  /* Inverse fg/bg gives the canonical "popover breaks out of the page" feel.
     Each language gets the right inverse pair from its own tokens. */
  background: var(--sy-color-fg);
  color: var(--sy-color-bg);
  font-family: var(--sy-font-body);
  font-size: 0.75rem;
  font-weight: 500;
  letter-spacing: -0.005em;
  padding: 5px 9px;
  border-radius: var(--sy-radius-sm);
  box-shadow: var(--sy-shadow-2);
  pointer-events: none;
  /* Short content stays single-line because it fits inside max-width; longer
     content wraps. `nowrap` would force overflow past the bg box. */
  max-width: 280px;
  line-height: 1.4;
  word-break: break-word;
}

.sy-tooltip-enter-active,
.sy-tooltip-leave-active {
  transition: opacity 120ms ease, transform 140ms var(--sy-motion-spring);
}
.sy-tooltip-enter-from,
.sy-tooltip-leave-to {
  opacity: 0;
}
.sy-tooltip--top.sy-tooltip-enter-from,
.sy-tooltip--top.sy-tooltip-leave-to    { transform: translateY(4px); }
.sy-tooltip--bottom.sy-tooltip-enter-from,
.sy-tooltip--bottom.sy-tooltip-leave-to { transform: translateY(-4px); }
.sy-tooltip--left.sy-tooltip-enter-from,
.sy-tooltip--left.sy-tooltip-leave-to   { transform: translateX(4px); }
.sy-tooltip--right.sy-tooltip-enter-from,
.sy-tooltip--right.sy-tooltip-leave-to  { transform: translateX(-4px); }

/* Self-attr selector for when `applyAnchorTheme` sets data-language directly
   on the tooltip element — otherwise the generic `[data-language="ambient"]`
   rule in tokens/index.css would paint the tooltip with the page gradient. */
.sy-tooltip[data-language="ambient"] {
  background: var(--sy-color-fg);
  color: var(--sy-color-bg);
}
</style>

<style scoped>
.sy-tooltip__anchor {
  display: inline-flex;
}
</style>
