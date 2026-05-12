<!--
  Developer input variant — sharp 4px corners, 28px md height, 1px inset
  outline on focus (chips into the input rather than blooming around it).
  Monospace prefix/suffix slots. Internal: consumers import `SyInput`.
-->
<script setup lang="ts">
import type { InputType, InputSize } from "./types";

defineProps<{
  modelValue?: string | number;
  type: InputType;
  size: InputSize;
  placeholder?: string;
  invalid?: boolean;
  disabled?: boolean;
  readonly?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();
</script>

<template>
  <label
    class="field"
    :class="[`field--${size}`, invalid && 'field--invalid', disabled && 'field--disabled']"
  >
    <span v-if="$slots.prefix" class="field__addon"><slot name="prefix" /></span>
    <input
      class="field__input"
      :type="type"
      :value="modelValue"
      :placeholder="placeholder"
      :disabled="disabled"
      :readonly="readonly"
      @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
    />
    <span v-if="$slots.suffix" class="field__addon"><slot name="suffix" /></span>
  </label>
</template>

<style scoped>
.field {
  display: inline-flex;
  align-items: center;
  gap: var(--sy-space-2);
  background: var(--sy-color-surface-2);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius);
  padding: 0 var(--sy-space-2);
  transition: border-color var(--sy-motion-fast),
              background var(--sy-motion-fast);
  min-width: 0;
}
.field--sm { border-radius: var(--sy-radius-sm); }

.field:hover:not(.field--invalid):not(.field--disabled) {
  border-color: var(--sy-color-fg-5);
  background: var(--sy-color-surface-3);
}
.field:focus-within {
  border-color: var(--sy-color-accent);
  background: var(--sy-color-surface-1);
  outline: 1px solid var(--sy-color-accent);
  outline-offset: -1px;
}
.field--invalid {
  border-color: var(--sy-color-bad);
}
.field--invalid:focus-within {
  outline-color: var(--sy-color-bad);
}
.field--disabled {
  opacity: 0.5;
}

.field__input {
  flex: 1;
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--sy-color-fg);
  font-family: var(--sy-font-body);
  font-size: 0.8125rem;
  letter-spacing: 0;
  outline: 0;
  height: 28px;
}
.field--sm .field__input { height: 22px; font-size: 0.75rem; }
.field--lg .field__input { height: 34px; font-size: 0.875rem; }
.field__input::placeholder { color: var(--sy-color-fg-4); }
.field__input:disabled { cursor: not-allowed; }

.field__addon {
  display: inline-flex;
  align-items: center;
  color: var(--sy-color-fg-3);
  font-size: 0.75rem;
  font-family: var(--sy-font-numeric);
  flex-shrink: 0;
}
</style>
