<!--
  SyListRow — horizontal row primitive for entity lists.

  Three named slots define the canonical row layout:
    - `leading`  — a dot, icon, or avatar (max-content column)
    - default   — the row's main content; stacks vertically if multiple
                  children, so consumers can pass title + subtitle directly
    - `trailing` — badges, values, chevrons (max-content column)

  Token-driven, no variant components. Per-language differences (corner
  radius, surface tint, ambient backdrop-blur) all flow through tokens.

  Three density steps (`compact` / `comfortable` / `spacious`) correspond to
  single-line, two-line, and three-line typical use. The component itself
  doesn't enforce line counts — the density only controls padding.

  Group rendering: when stacked in a list, consumers turn off `bordered`
  on the inner rows and put a single border around the group, or apply
  `border-radius` to the first/last row themselves. A SyListGroup helper
  that handles this is a future composite.

  Interactive rows get a hover background and a quick scale-down on press.
  Deliberately simpler than SySurface's cursor-tracking treatment — list
  rows are dense, and the fancier effect would feel overdone at row scale.
-->

<script setup lang="ts">
import { computed } from "vue";

type Density = "compact" | "comfortable" | "spacious";
type AsTag = "div" | "a" | "button" | "li";

const props = withDefaults(
  defineProps<{
    /** HTML tag. Use `a` for navigation rows, `button` for action rows. */
    as?: AsTag;
    density?: Density;
    /** Adds hover/active treatment + cursor:pointer. */
    interactive?: boolean;
    /** Whether to render the 1px border and outer radius. Off for grouped rows. */
    bordered?: boolean;
    /** Native `href` when `as="a"`. */
    href?: string;
    /** Visually mark the row as the active selection (e.g., its detail
        rail is open). Adds an accent inset bar + tinted bg. */
    selected?: boolean;
  }>(),
  {
    as: "div",
    density: "comfortable",
    interactive: false,
    bordered: true,
    selected: false,
  },
);

const classes = computed(() => [
  "sy-listrow",
  `sy-listrow--${props.density}`,
  props.bordered && "sy-listrow--bordered",
  props.interactive && "sy-listrow--interactive",
  props.selected && "sy-listrow--selected",
]);
</script>

<template>
  <component
    :is="as"
    :href="as === 'a' ? href : undefined"
    :class="classes"
  >
    <span v-if="$slots.leading" class="sy-listrow__leading">
      <slot name="leading" />
    </span>
    <span class="sy-listrow__main">
      <slot />
    </span>
    <span v-if="$slots.trailing" class="sy-listrow__trailing">
      <slot name="trailing" />
    </span>
  </component>
</template>

<style scoped>
.sy-listrow {
  display: grid;
  /* The two `max-content` columns shrink to fit their slot content; the `1fr`
     main column takes the rest. If a slot is empty (v-if removes the wrapper),
     the grid collapses gracefully. */
  grid-template-columns: max-content 1fr max-content;
  gap: var(--sy-space-3);
  align-items: center;
  background: var(--sy-color-surface-1);
  color: var(--sy-color-fg);
  text-decoration: none;
  transition: background var(--sy-motion-fast),
              border-color var(--sy-motion-fast),
              transform var(--sy-motion-fast);

  /* Reset native styles for the `button` / `a` variants so the row looks
     identical regardless of element. Without these, `<button>` ships its
     own dark border, centered text, and intrinsic (content) width — which
     made the row appear with a black rim and all its content compressed
     to the left of the parent surface. */
  width: 100%;
  border: 0;
  margin: 0;
  font: inherit;
  text-align: left;
  appearance: none;
  -webkit-appearance: none;
}

.sy-listrow--bordered {
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-lg);
}

.sy-listrow--compact     { padding: var(--sy-space-2) var(--sy-space-3); }
.sy-listrow--comfortable { padding: var(--sy-space-3) var(--sy-space-4); }
.sy-listrow--spacious    { padding: var(--sy-space-4) var(--sy-space-5); }

.sy-listrow--interactive {
  cursor: pointer;
}
.sy-listrow--interactive:hover {
  background: var(--sy-color-surface-2);
  border-color: var(--sy-color-fg-5);
}
.sy-listrow--interactive:active {
  background: var(--sy-color-surface-3);
  transform: scale(0.997);
}
.sy-listrow--interactive:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}

/* Selected: a 3px accent stripe along the leading edge plus a faint
   accent-tinted bg. Layered as box-shadow inset so it doesn't push the
   content (and works whether the row is bordered or not). */
.sy-listrow--selected {
  background: var(--sy-color-accent-subtle);
  box-shadow: inset 3px 0 0 0 var(--sy-color-accent);
}
.sy-listrow--selected:hover {
  background: var(--sy-color-accent-subtle);
}

.sy-listrow__leading {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
}

.sy-listrow__main {
  /* `min-width: 0` is critical for grid children that contain long text —
     without it, the `1fr` column grows to the intrinsic text width and
     pushes the trailing slot off-screen. */
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.sy-listrow__trailing {
  display: inline-flex;
  align-items: center;
  gap: var(--sy-space-2);
  color: var(--sy-color-fg-3);
  flex-shrink: 0;
}

/* Note: no ambient backdrop-filter here. Applying `backdrop-filter` to
   every row created visible per-row "glass tiles" — the filter applies to
   the row's element rect even when bg is transparent, so inter-row gaps
   (padding, margins) showed the page gradient unfiltered while the rows
   showed a blurred-saturated patch. The right scope for the glass effect
   is the bounding surface (SySurface, or a story wrapper) above the rows,
   so it composites once across the whole region. SyListRow inherits that
   filter's result like any other content. */
</style>
