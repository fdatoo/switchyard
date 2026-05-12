<!--
  SyDot — standalone status dot.

  The badge-internal dot's primitive form. Use anywhere you need a small
  semantic-color circle without surrounding chip chrome: sidebar live
  indicators, list-row leading marks, breadcrumb daemon-status indicator,
  inline event-type pips.

  Same pulse vocabulary as SyBadge — `slow` for steady-alive, `fast` for
  in-flight. Both honor `prefers-reduced-motion`.
-->
<script setup lang="ts">
import { computed } from "vue";

type Intent = "neutral" | "accent" | "good" | "warn" | "bad" | "info" | "purple";
type Size = "sm" | "md" | "lg";
type Pulse = "off" | "slow" | "fast";

const props = withDefaults(
  defineProps<{
    intent?: Intent;
    size?: Size;
    pulse?: Pulse;
    /** Accessible label. Default is the intent name; override when meaning is contextual. */
    label?: string;
  }>(),
  { intent: "neutral", size: "md", pulse: "off" },
);

const classes = computed(() => [
  "sy-dot",
  `sy-dot--intent-${props.intent}`,
  `sy-dot--${props.size}`,
  props.pulse !== "off" ? `sy-dot--pulse-${props.pulse}` : null,
]);
</script>

<template>
  <span :class="classes" role="img" :aria-label="label ?? intent" />
</template>

<style scoped>
.sy-dot {
  position: relative;
  display: inline-block;
  border-radius: 999px;
  background: currentColor;
  flex-shrink: 0;
}
.sy-dot--sm { width: 6px;  height: 6px;  }
.sy-dot--md { width: 8px;  height: 8px;  }
.sy-dot--lg { width: 10px; height: 10px; }

.sy-dot--intent-neutral { color: var(--sy-color-fg-4); }
.sy-dot--intent-accent  { color: var(--sy-color-accent); }
.sy-dot--intent-good    { color: var(--sy-color-good); }
.sy-dot--intent-warn    { color: var(--sy-color-warn); }
.sy-dot--intent-bad     { color: var(--sy-color-bad); }
.sy-dot--intent-info    { color: var(--sy-color-info); }
.sy-dot--intent-purple  { color: var(--sy-color-purple); }

/* Slow: opacity breathe. Conveys steady-alive. */
.sy-dot--pulse-slow {
  animation: sy-dot-breathe 2.4s ease-in-out infinite;
}

/* Fast: expanding halo on ::before. Dot itself stays at full opacity so
   status is always visible; halo conveys in-flight activity. */
.sy-dot--pulse-fast::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: 999px;
  background: currentColor;
  animation: sy-dot-halo 1.5s cubic-bezier(0, 0, 0.2, 1) infinite;
}

@keyframes sy-dot-breathe {
  0%, 100% { opacity: 1; }
  50%      { opacity: 0.4; }
}

@keyframes sy-dot-halo {
  0%   { transform: scale(1);   opacity: 0.75; }
  100% { transform: scale(2.8); opacity: 0;    }
}

@media (prefers-reduced-motion: reduce) {
  .sy-dot--pulse-slow,
  .sy-dot--pulse-fast::before {
    animation: none;
  }
}
</style>
