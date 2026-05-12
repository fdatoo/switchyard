<!--
  Ambient input variant — rounded-xl, 44px+ touch height, glass background
  with backdrop-blur + saturate. Larger accent ring on focus. Internal:
  consumers import `SyInput`.
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
  background: var(--sy-color-surface-1);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-xl);
  padding: 0 var(--sy-space-4);
  -webkit-backdrop-filter: blur(20px) saturate(140%);
  backdrop-filter: blur(20px) saturate(140%);
  transition: border-color var(--sy-motion),
              box-shadow var(--sy-motion),
              background var(--sy-motion);
  min-width: 0;
}
.field--sm { padding: 0 var(--sy-space-3); }
.field--lg { padding: 0 var(--sy-space-5); }

.field:hover:not(.field--invalid):not(.field--disabled) {
  background: var(--sy-color-surface-2);
}
.field:focus-within {
  border-color: var(--sy-color-accent);
  box-shadow: 0 0 0 4px var(--sy-color-accent-subtle);
}
.field--invalid {
  border-color: var(--sy-color-bad);
}
.field--invalid:focus-within {
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--sy-color-bad) 22%, transparent);
}
.field--disabled {
  opacity: 0.45;
}

.field__input {
  flex: 1;
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--sy-color-fg);
  font-family: var(--sy-font-body);
  font-size: 1rem;
  outline: 0;
  height: 44px;
}
.field--sm .field__input { height: 36px; font-size: 0.9375rem; }
.field--lg .field__input { height: 56px; font-size: 1.125rem; }
.field__input::placeholder { color: var(--sy-color-fg-4); }
.field__input:disabled { cursor: not-allowed; }

.field__addon {
  display: inline-flex;
  align-items: center;
  color: var(--sy-color-fg-3);
  font-size: 0.9375rem;
  flex-shrink: 0;
}
</style>
