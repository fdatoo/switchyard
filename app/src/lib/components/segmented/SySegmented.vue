<!--
  SySegmented — single-select pill-bar.

  One of N options is always selected (mutually-exclusive). Renders as
  an inset track containing pill children; the active pill rides on
  top of the track with surface-1 + accent fill. Used wherever an iOS-
  style segmented control fits (mode toggle, time-window picker, etc.).

  Generic over the option `id` type so callers can use string literal
  unions or branded ids without losing type safety.

  Accessibility: rendered as `role="radiogroup"` with `role="radio"` +
  `aria-checked` on each pill. Disabled bar (when `disabled` is true)
  sets `aria-disabled` on the group and disables every child button.

  Visual treatment matches the Mode picker in settings: orange-filled
  active pill with white text in friendly mode, lifted via
  `--sy-shadow`. Inactive pills are transparent and inherit text color.
-->
<script setup lang="ts" generic="T extends string">
import { computed } from "vue";

interface SegmentedOption {
  id: T;
  label: string;
}

const props = withDefaults(
  defineProps<{
    /** Currently-selected option id. */
    modelValue: T;
    /** Available choices, rendered left-to-right. */
    options: readonly SegmentedOption[];
    /** Optional accessible label for the group. */
    ariaLabel?: string;
    /** Disables every pill at once and dims the bar. */
    disabled?: boolean;
  }>(),
  { disabled: false },
);

const emit = defineEmits<{
  "update:modelValue": [value: T];
}>();

const isActive = computed(() => (id: T): boolean => id === props.modelValue);

function onSelect(id: T): void {
  if (props.disabled || id === props.modelValue) return;
  emit("update:modelValue", id);
}
</script>

<template>
  <div
    class="sy-seg"
    role="radiogroup"
    :aria-label="ariaLabel"
    :aria-disabled="disabled || undefined"
  >
    <button
      v-for="opt in options"
      :key="opt.id"
      type="button"
      role="radio"
      :aria-checked="isActive(opt.id)"
      :class="['sy-seg__pill', isActive(opt.id) && 'sy-seg__pill--active']"
      :disabled="disabled"
      @click="onSelect(opt.id)"
    >
      {{ opt.label }}
    </button>
  </div>
</template>

<style scoped>
.sy-seg {
  display: inline-flex;
  padding: 3px;
  background: var(--sy-color-surface-2);
  border-radius: 999px;
  border: 1px solid var(--sy-color-line-soft);
  flex-shrink: 0;
}
.sy-seg[aria-disabled="true"] {
  opacity: 0.55;
}

.sy-seg__pill {
  appearance: none;
  -webkit-appearance: none;
  border: 0;
  background: transparent;
  color: var(--sy-color-fg-2);
  font: inherit;
  font-weight: 500;
  font-size: 0.8125rem;
  padding: 6px 14px;
  border-radius: 999px;
  cursor: pointer;
  white-space: nowrap;
  transition: background var(--sy-motion-fast),
              color var(--sy-motion-fast),
              box-shadow var(--sy-motion-fast);
}
.sy-seg__pill:hover:not(:disabled):not(.sy-seg__pill--active) {
  color: var(--sy-color-fg);
}
.sy-seg__pill:disabled {
  cursor: not-allowed;
}
.sy-seg__pill--active {
  background: var(--sy-color-accent);
  color: #fff;
  box-shadow: var(--sy-shadow);
}
.sy-seg__pill--active:hover:not(:disabled) {
  background: var(--sy-color-accent-2);
}
.sy-seg__pill:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}
</style>
