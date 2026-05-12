<!--
  SyText — typographic primitive.

  Token-driven, no variant components. All visual values (font family, size,
  weight, color) reference --sy-* tokens, so a language switch automatically
  re-renders text with the right type and palette.

  The default `as="div"` is intentional: text is a block by default so that
  vertically-stacked SyText elements lay out as separate lines without
  needing a wrapping flex/grid container. Use `as="span"` when inline.

  Use `variant` to pick the typographic role (display/title/body/label/…).
  Use `tone` to pick the semantic color (default/muted/accent/good/warn/bad).
  Use `weight` only to override the variant's intrinsic weight.
-->
<script setup lang="ts">
import { computed } from "vue";

type AsTag = "h1" | "h2" | "h3" | "h4" | "p" | "span" | "div" | "label";
type Variant = "display" | "title" | "subtitle" | "body" | "label" | "overline" | "caption" | "numeric";
type Tone = "default" | "muted" | "subtle" | "accent" | "good" | "warn" | "bad";
type Weight = "normal" | "medium" | "semibold";

const props = withDefaults(
  defineProps<{
    /** Semantic tag to render. Default `div` (block, semantically neutral). */
    as?: AsTag;
    /** Typographic role. Drives size, weight, line-height, letter-spacing. */
    variant?: Variant;
    /** Semantic color. Maps to a `--sy-color-fg*` or signal-color token. */
    tone?: Tone;
    /** Override the variant's intrinsic weight. Rarely needed. */
    weight?: Weight;
  }>(),
  {
    as: "div",
    variant: "body",
    tone: "default",
  },
);

const classes = computed(() => [
  "sy-text",
  `sy-text--${props.variant}`,
  `sy-text--tone-${props.tone}`,
  props.weight ? `sy-text--weight-${props.weight}` : null,
]);
</script>

<template>
  <component :is="as" :class="classes">
    <slot />
  </component>
</template>

<style scoped>
.sy-text {
  margin: 0;
  font-family: var(--sy-font-body);
  letter-spacing: -0.005em;
}
.sy-text--display {
  font-size: 1.75rem;
  font-weight: 600;
  letter-spacing: -0.025em;
  line-height: 1.15;
}
.sy-text--title {
  font-size: 1.25rem;
  font-weight: 600;
  letter-spacing: -0.015em;
  line-height: 1.25;
}
.sy-text--subtitle {
  font-size: 1rem;
  font-weight: 500;
  line-height: 1.4;
}
.sy-text--body {
  font-size: 0.9375rem;
  line-height: 1.5;
}
.sy-text--label {
  font-size: 0.6875rem;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  font-weight: 500;
}
.sy-text--overline {
  font-size: 0.625rem;
  text-transform: uppercase;
  letter-spacing: 0.12em;
  font-weight: 600;
}
.sy-text--caption {
  font-size: 0.8125rem;
  line-height: 1.4;
}
.sy-text--numeric {
  font-family: var(--sy-font-numeric);
  font-feature-settings: var(--sy-numeric-feature);
  font-size: 0.875rem;
}

.sy-text--tone-default { color: var(--sy-color-fg); }
.sy-text--tone-muted   { color: var(--sy-color-fg-2); }
.sy-text--tone-subtle  { color: var(--sy-color-fg-3); }
.sy-text--tone-accent  { color: var(--sy-color-accent); }
.sy-text--tone-good    { color: var(--sy-color-good); }
.sy-text--tone-warn    { color: var(--sy-color-warn); }
.sy-text--tone-bad     { color: var(--sy-color-bad); }

.sy-text--weight-normal   { font-weight: 400; }
.sy-text--weight-medium   { font-weight: 500; }
.sy-text--weight-semibold { font-weight: 600; }
</style>
