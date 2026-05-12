<!--
  SyCheckbox — boolean input rendered as a styled checkbox.

  Token-driven, no variant components. The visual checkbox is a styled
  <span> overlaying a visually-hidden real <input type="checkbox">, so
  screen readers, keyboard navigation, focus order, and form submission
  all work natively. Clicking anywhere on the wrapping <label> (visual
  box or text) toggles the input.

  Use for discrete selections in forms ("I agree", multi-select rows).
  Use SySwitch for instantly-applied settings.

  Supports a three-state mode via `indeterminate` — the box renders a
  horizontal bar instead of a check, the canonical "partially selected"
  affordance for parent-of-multiselect patterns. Indeterminate is a DOM
  property (not an HTML attribute), so we sync it via a watcher.
-->
<script setup lang="ts">
import { computed, ref, watch } from "vue";

type Size = "sm" | "md" | "lg";

const props = withDefaults(
  defineProps<{
    modelValue?: boolean;
    /** Tri-state: render a horizontal bar instead of a check. */
    indeterminate?: boolean;
    disabled?: boolean;
    size?: Size;
    /** Renders the invalid styling (red border). */
    invalid?: boolean;
  }>(),
  {
    modelValue: false,
    indeterminate: false,
    disabled: false,
    size: "md",
    invalid: false,
  },
);

const emit = defineEmits<{
  "update:modelValue": [value: boolean];
}>();

const inputRef = ref<HTMLInputElement | null>(null);

/* `indeterminate` is a DOM property without a matching HTML attribute, so we
   sync it imperatively whenever the prop changes (and on mount). */
watch(
  () => props.indeterminate,
  (v) => {
    if (inputRef.value) inputRef.value.indeterminate = v;
  },
  { immediate: true },
);

function onChange(e: Event): void {
  emit("update:modelValue", (e.target as HTMLInputElement).checked);
}

const classes = computed(() => [
  "sy-checkbox",
  `sy-checkbox--${props.size}`,
  props.disabled && "sy-checkbox--disabled",
  props.invalid && "sy-checkbox--invalid",
]);
</script>

<template>
  <label :class="classes">
    <input
      ref="inputRef"
      type="checkbox"
      class="sy-checkbox__input"
      :checked="modelValue"
      :disabled="disabled"
      @change="onChange"
    />
    <span class="sy-checkbox__box" aria-hidden="true">
      <svg v-if="!indeterminate" viewBox="0 0 12 12" class="sy-checkbox__check">
        <path
          d="M2.5 6.5 L5 9 L9.5 3.5"
          fill="none"
          stroke="currentColor"
          stroke-width="1.75"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
      <span v-else class="sy-checkbox__bar" />
    </span>
    <span v-if="$slots.default" class="sy-checkbox__label">
      <slot />
    </span>
  </label>
</template>

<style scoped>
.sy-checkbox {
  display: inline-flex;
  align-items: center;
  gap: var(--sy-space-2);
  cursor: pointer;
  user-select: none;
  font-family: var(--sy-font-body);
}
.sy-checkbox--disabled { cursor: not-allowed; opacity: 0.5; }

/* The real input is visually hidden but kept in the layout flow so that
   keyboard focus, form submission, and screen readers all work. */
.sy-checkbox__input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
  pointer-events: none;
  margin: 0;
}

.sy-checkbox__box {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-sm);
  background: var(--sy-color-surface-1);
  /* color: transparent hides the check (which uses currentColor as its
     stroke) until the input is :checked, when the rule below paints it. */
  color: transparent;
  flex-shrink: 0;
  transition: border-color var(--sy-motion-fast),
              background var(--sy-motion-fast),
              color var(--sy-motion-fast);
}
.sy-checkbox--sm .sy-checkbox__box { width: 14px; height: 14px; }
.sy-checkbox--lg .sy-checkbox__box { width: 20px; height: 20px; }

.sy-checkbox:hover:not(.sy-checkbox--disabled) .sy-checkbox__box {
  border-color: var(--sy-color-fg-4);
}

/* The hidden input still receives focus; we proxy its focus ring onto the
   visible box via the adjacent-sibling selector. */
.sy-checkbox__input:focus-visible + .sy-checkbox__box {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}

.sy-checkbox__input:checked + .sy-checkbox__box,
.sy-checkbox__input:indeterminate + .sy-checkbox__box {
  background: var(--sy-color-accent);
  border-color: var(--sy-color-accent);
  color: var(--sy-color-bg);
}

.sy-checkbox__check {
  width: 75%;
  height: 75%;
}

.sy-checkbox__bar {
  width: 60%;
  height: 2px;
  background: currentColor;
  border-radius: 999px;
}

.sy-checkbox--invalid .sy-checkbox__box {
  border-color: var(--sy-color-bad);
}

.sy-checkbox__label {
  font-size: 0.9375rem;
  color: var(--sy-color-fg);
}
.sy-checkbox--sm .sy-checkbox__label { font-size: 0.8125rem; }
.sy-checkbox--lg .sy-checkbox__label { font-size: 1rem; }
</style>
