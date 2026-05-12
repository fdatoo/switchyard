<!--
  SySurface — the card/panel primitive.

  Token-driven, no variant components. Per-language differences (radius,
  shadow, border, ambient backdrop-blur) all flow through tokens or a single
  `[data-language="ambient"]` descendant rule for the blur effect.

  Interactive surfaces (`interactive` prop) get a cursor-tracking radial
  highlight on hover and a press-toward-cursor animation: clicking the top-
  right corner makes the surface lean a few pixels that way and scale to
  98.5%, releasing with a spring curve. The translation is cursor-relative
  but capped (±4px) so the effect is felt rather than seen on large cards.
  All press effects degrade gracefully — keyboard activation still triggers
  the scale-down via :active, and the hover highlight is hover-media-gated
  so touch devices don't get a stuck highlight.
-->
<script setup lang="ts">
import { computed, ref } from "vue";

type Elevation = "flat" | "raised" | "elevated";
type Padding = "none" | "sm" | "md" | "lg";
type AsTag = "div" | "section" | "article" | "aside" | "a" | "button";

const props = withDefaults(
  defineProps<{
    /** HTML tag to render. Use `a` or `button` for natively-interactive surfaces. */
    as?: AsTag;
    /** Shadow depth. `flat` = none, `raised` = card default, `elevated` = sheet/modal. */
    elevation?: Elevation;
    /** Inner padding. Use `none` when the content provides its own padding. */
    padding?: Padding;
    /**
     * Enables the cursor-tracking hover highlight and press-lean animation.
     * Also sets `cursor: pointer`. Use for clickable cards / list rows.
     */
    interactive?: boolean;
    /** Whether to render the 1px border. Off for nested surfaces or trim. */
    bordered?: boolean;
  }>(),
  {
    as: "div",
    elevation: "raised",
    padding: "md",
    interactive: false,
    bordered: true,
  },
);

const root = ref<HTMLElement | null>(null);

/**
 * Track the cursor position as CSS custom properties on the element. The
 * `::after` radial-gradient highlight reads these to render at the cursor.
 */
function onMove(e: PointerEvent): void {
  const el = root.value;
  if (!el) return;
  const r = el.getBoundingClientRect();
  el.style.setProperty("--sy-mouse-x", `${((e.clientX - r.left) / r.width) * 100}%`);
  el.style.setProperty("--sy-mouse-y", `${((e.clientY - r.top) / r.height) * 100}%`);
}

/**
 * On press, compute the cursor's offset from the surface's center and apply
 * a small translation toward the cursor. Multiplied by 3 and capped at ±4px:
 * pressing the corner of a huge card shouldn't lean dramatically, just
 * subtly. Combined with the `:active` scale rule, this is the "lean into the
 * click" feel.
 */
function onDown(e: PointerEvent): void {
  const el = root.value;
  if (!el) return;
  const r = el.getBoundingClientRect();
  const dx = (e.clientX - (r.left + r.width / 2)) / (r.width / 2);
  const dy = (e.clientY - (r.top + r.height / 2)) / (r.height / 2);
  const cap = 4;
  el.style.setProperty("--sy-press-tx", `${Math.max(-cap, Math.min(cap, dx * 3))}px`);
  el.style.setProperty("--sy-press-ty", `${Math.max(-cap, Math.min(cap, dy * 3))}px`);
}

function onRelease(): void {
  const el = root.value;
  if (!el) return;
  el.style.setProperty("--sy-press-tx", "0px");
  el.style.setProperty("--sy-press-ty", "0px");
}

const classes = computed(() => [
  "sy-surface",
  `sy-surface--elev-${props.elevation}`,
  `sy-surface--pad-${props.padding}`,
  props.bordered ? "sy-surface--bordered" : null,
  props.interactive ? "sy-surface--interactive" : null,
]);
</script>

<template>
  <component
    :is="as"
    ref="root"
    :class="classes"
    @pointermove="interactive ? onMove($event) : undefined"
    @pointerdown="interactive ? onDown($event) : undefined"
    @pointerup="interactive ? onRelease() : undefined"
    @pointerleave="interactive ? onRelease() : undefined"
  >
    <slot />
  </component>
</template>

<style scoped>
.sy-surface {
  background: var(--sy-color-surface-1);
  border-radius: var(--sy-radius-lg);
  /* Clip children at the corner radius. Without this, an interactive child
     (e.g., a hover-tinted SyListRow) fills the corner area inside the
     border and visually "fills in" the rounded corners. The interactive
     hover ::after and the press transform on this surface aren't affected
     by the clip (the pseudo uses `border-radius: inherit` and the transform
     moves the entire surface). */
  overflow: hidden;
  transition: background var(--sy-motion-fast),
              border-color var(--sy-motion-fast),
              box-shadow var(--sy-motion);
}

.sy-surface--bordered {
  border: 1px solid var(--sy-color-line);
}

.sy-surface--elev-flat { box-shadow: none; }
.sy-surface--elev-raised { box-shadow: var(--sy-shadow); }
.sy-surface--elev-elevated { box-shadow: var(--sy-shadow-elevated); }

.sy-surface--pad-none { padding: 0; }
.sy-surface--pad-sm { padding: var(--sy-space-3); }
.sy-surface--pad-md { padding: var(--sy-space-4); }
.sy-surface--pad-lg { padding: var(--sy-space-5); }

.sy-surface--interactive {
  position: relative;
  cursor: pointer;
  /* Translate variables are set by onDown/onRelease; default to 0px so the
     surface stays still when not pressed. */
  transform: translate(var(--sy-press-tx, 0px), var(--sy-press-ty, 0px))
             scale(var(--sy-press-scale, 1));
  transition: transform 280ms var(--sy-motion-spring),
              border-color var(--sy-motion-fast),
              box-shadow var(--sy-motion);
}

/* Hover highlight: a radial-gradient pseudo following the cursor. Hover-gated
   so touch devices don't get a stuck mouse-position blob. */
@media (hover: hover) {
  .sy-surface--interactive::after {
    content: "";
    position: absolute;
    inset: 0;
    pointer-events: none;
    border-radius: inherit;
    opacity: 0;
    background: radial-gradient(
      300px circle at var(--sy-mouse-x, 50%) var(--sy-mouse-y, 50%),
      var(--sy-color-accent-subtle),
      transparent 55%
    );
    transition: opacity 220ms ease;
  }
  .sy-surface--interactive:hover::after { opacity: 1; }
}

.sy-surface--interactive:hover {
  border-color: var(--sy-color-fg-5);
  box-shadow: var(--sy-shadow-2);
}

/* :active fires on both pointer and keyboard activation (Space/Enter), so
   keyboard users still get the press feedback even without pointer events. */
.sy-surface--interactive:active {
  --sy-press-scale: 0.985;
  transition: transform 80ms ease-out,
              border-color var(--sy-motion-fast),
              box-shadow var(--sy-motion);
}

.sy-surface--interactive:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}

/* Ambient surfaces are intentionally glassy: their surface colors are
   translucent rgba values in `tokens/ambient.css`. The blur is scoped here
   (rather than on the token) so other languages don't pay the compositing
   cost for a no-op effect on opaque surfaces. */
[data-language="ambient"] .sy-surface {
  -webkit-backdrop-filter: blur(20px) saturate(140%);
  backdrop-filter: blur(20px) saturate(140%);
}
</style>
