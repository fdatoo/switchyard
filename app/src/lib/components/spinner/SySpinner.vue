<!--
  SySpinner — indeterminate loading indicator.

  A rotating SVG arc using `stroke-dasharray` for the visible segment. Color
  flows in via `currentColor` so the spinner picks up the parent's CSS
  color, the same pattern as SyIcon. Sized in px (default 18); for arbitrary
  sizes pass any number.

  Includes `role="status"` and an `aria-label` so screen readers announce
  the loading state. The label defaults to "Loading…" but can be overridden
  for context-specific wording.

  No determinate-progress mode — when you need that, build a SyProgress
  primitive. This is strictly for "something is happening, don't know how
  long it'll take."
-->
<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    /** Pixel size of the spinner. Default 18. */
    size?: number | string;
    /** Stroke width of the arc. Default scales with size. */
    strokeWidth?: number | string;
    /** Screen-reader label. */
    label?: string;
  }>(),
  { size: 18, label: "Loading…" },
);

const stroke = computed(() => {
  if (props.strokeWidth != null) return props.strokeWidth;
  const px = typeof props.size === "number" ? props.size : parseFloat(String(props.size));
  /* Keep stroke proportional to size: ~12% of diameter, with a sensible floor. */
  return Math.max(1.5, Math.round(px * 0.12 * 10) / 10);
});
</script>

<template>
  <span class="sy-spinner" role="status" :aria-label="label">
    <svg :width="size" :height="size" viewBox="0 0 24 24" aria-hidden="true">
      <circle
        cx="12"
        cy="12"
        r="9"
        fill="none"
        stroke="currentColor"
        :stroke-width="stroke"
        stroke-linecap="round"
        stroke-dasharray="14 100"
      />
    </svg>
  </span>
</template>

<style scoped>
.sy-spinner {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  /* No `color:` rule — the spinner inherits from its parent. Same pattern as
     SyIcon. To tint the spinner, set the parent's color (e.g., put it inside
     a button, an EmptyState, or wrap in `<span style="color: …">`). */
}

.sy-spinner svg {
  animation: sy-spinner-rotate 900ms linear infinite;
}

@keyframes sy-spinner-rotate {
  to { transform: rotate(360deg); }
}

@media (prefers-reduced-motion: reduce) {
  .sy-spinner svg {
    /* Pulse opacity instead of spinning, so loading is still signaled. */
    animation: sy-spinner-pulse 1.4s ease-in-out infinite;
  }
  @keyframes sy-spinner-pulse {
    0%, 100% { opacity: 0.4; }
    50%      { opacity: 1;   }
  }
}
</style>
