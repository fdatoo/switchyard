<!--
  SyBadge — status pill / chip.

  Pill-shaped in every language. The per-language differences (corner radius
  on adjacent components, surface tint) are handled by tokens; the badge
  itself is the canonical "pill regardless of language" exception, matching
  how Linear, Stripe, and Apple keep status chips pill-shaped even in their
  otherwise-sharp UIs.

  Color story: each `intent` selects a base token color into the `--b-color`
  custom property. The three `appearance` modes derive their fill, fg, and
  border from that single base color via `color-mix()`, so we don't need to
  define `--sy-color-good-subtle`, `--sy-color-warn-subtle`, etc. — a
  language pack only needs to override the base intent colors.

  The dot can `pulse` to convey aliveness: `slow` is a gentle opacity breathe
  (Running / steady-alive); `fast` is an expanding halo ring drawn with a
  `::before` pseudo (Reconnecting / in-flight). Both honor
  `prefers-reduced-motion`.
-->
<script setup lang="ts">
import { computed } from "vue";

type Intent = "neutral" | "accent" | "good" | "warn" | "bad" | "info" | "purple";
type Appearance = "soft" | "solid" | "outline";
type Size = "sm" | "md";
type Pulse = "off" | "slow" | "fast";

const props = withDefaults(
  defineProps<{
    /** Semantic color. Maps to a `--sy-color-*` token via `--b-color`. */
    intent?: Intent;
    /**
     * Visual treatment:
     *   - `soft` — tinted background, colored fg. The default; reads as a chip.
     *   - `solid` — full intent color background, canvas-color fg. Loudest.
     *   - `outline` — transparent bg with a tinted border. Quietest.
     */
    appearance?: Appearance;
    size?: Size;
    /** Whether to render the leading status dot. */
    dot?: boolean;
    /**
     * Dot animation:
     *   - `slow` — gentle breathe, for "alive" steady states like Running.
     *   - `fast` — expanding halo ring, for in-flight states like Reconnecting.
     *   - `off` (default) — static.
     */
    pulse?: Pulse;
  }>(),
  {
    intent: "neutral",
    appearance: "soft",
    size: "md",
    dot: false,
    pulse: "off",
  },
);

const classes = computed(() => [
  "sy-badge",
  `sy-badge--intent-${props.intent}`,
  `sy-badge--${props.appearance}`,
  `sy-badge--${props.size}`,
]);

const dotClasses = computed(() => [
  "sy-badge__dot",
  props.pulse !== "off" ? `sy-badge__dot--pulse-${props.pulse}` : null,
]);
</script>

<template>
  <span :class="classes">
    <span v-if="dot" :class="dotClasses" aria-hidden="true" />
    <slot />
  </span>
</template>

<style scoped>
.sy-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: var(--sy-radius-pill);
  border: 1px solid transparent;
  font-family: var(--sy-font-body);
  font-weight: 500;
  letter-spacing: 0;
  white-space: nowrap;
  line-height: 1;
}

.sy-badge--sm { font-size: 0.6875rem; padding: 2px 7px; }
.sy-badge--md { font-size: 0.75rem;   padding: 3px 9px; }

.sy-badge__dot {
  position: relative;
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: currentColor;
  flex-shrink: 0;
}

/* Slow: gentle breathe. Conveys steady-alive without grabbing attention. */
.sy-badge__dot--pulse-slow {
  animation: sy-badge-breathe 2.4s ease-in-out infinite;
}

/* Fast: an expanding halo ring drawn on a ::before pseudo. The dot itself
   stays at full opacity so the status is always visible; the halo conveys
   the in-flight activity. `currentColor` inherits the badge's intent color
   so the halo automatically matches. */
.sy-badge__dot--pulse-fast::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: 999px;
  background: currentColor;
  animation: sy-badge-halo 1.5s cubic-bezier(0, 0, 0.2, 1) infinite;
}

@keyframes sy-badge-breathe {
  0%, 100% { opacity: 1; }
  50%      { opacity: 0.4; }
}

@keyframes sy-badge-halo {
  0%   { transform: scale(1);   opacity: 0.75; }
  100% { transform: scale(2.8); opacity: 0;    }
}

@media (prefers-reduced-motion: reduce) {
  .sy-badge__dot--pulse-slow,
  .sy-badge__dot--pulse-fast::before {
    animation: none;
  }
}

/* Each intent declares its base color. `appearance` rules below derive fill,
   fg, and border from `--b-color` via `color-mix()`. */
.sy-badge--intent-neutral { --b-color: var(--sy-color-fg-3); }
.sy-badge--intent-accent  { --b-color: var(--sy-color-accent); }
.sy-badge--intent-good    { --b-color: var(--sy-color-good); }
.sy-badge--intent-warn    { --b-color: var(--sy-color-warn); }
.sy-badge--intent-bad     { --b-color: var(--sy-color-bad); }
.sy-badge--intent-info    { --b-color: var(--sy-color-info); }
.sy-badge--intent-purple  { --b-color: var(--sy-color-purple); }

.sy-badge--soft {
  color: var(--b-color);
  background: color-mix(in srgb, var(--b-color) 12%, transparent);
}
/* Neutral soft would render as muted-grey-on-muted-grey via color-mix; use
   the surface token directly for clearer separation. */
.sy-badge--soft.sy-badge--intent-neutral {
  background: var(--sy-color-surface-2);
  color: var(--sy-color-fg-2);
  border-color: var(--sy-color-line);
}

.sy-badge--solid {
  color: var(--sy-color-bg);
  background: var(--b-color);
}

.sy-badge--outline {
  color: var(--b-color);
  background: transparent;
  border-color: color-mix(in srgb, var(--b-color) 45%, transparent);
}
.sy-badge--outline.sy-badge--intent-neutral {
  color: var(--sy-color-fg-3);
  border-color: var(--sy-color-line);
}
</style>
