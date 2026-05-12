<!--
  SyInput — variant dispatcher for single-line text inputs.

  Three variants: friendly is rounded-lg with a soft accent ring on focus,
  developer is sharp with a 1px inset outline, ambient is rounded-xl with
  a 44px+ touch target and backdrop-blur. The shape and focus treatments
  differ enough across languages that tokens alone can't bridge them.

  v-model and slot forwarding: the dispatcher accepts the canonical input
  contract (`modelValue` + `update:modelValue`, plus `prefix`/`suffix` slots)
  and re-emits to the variant. `inheritAttrs: false` keeps extra attributes
  (autocomplete, name, aria-*, etc.) from being double-applied; they get
  forwarded explicitly via `v-bind="$attrs"`.
-->
<script setup lang="ts">
import { computed } from "vue";
import { useLanguageStore } from "@/lib/theme/language-store";
import { resolveVariant } from "@/lib/theme/variant-registry";
import type { InputType, InputSize } from "./types";

defineOptions({ inheritAttrs: false });

defineProps<{
  modelValue?: string | number;
  /** Native input type. Constrained to single-line text-ish inputs. */
  type?: InputType;
  size?: InputSize;
  placeholder?: string;
  /** Renders the invalid styling (red border, red focus ring). */
  invalid?: boolean;
  disabled?: boolean;
  readonly?: boolean;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const store = useLanguageStore();
const VariantComponent = computed(() => resolveVariant("Input", store.language));
</script>

<template>
  <component
    :is="VariantComponent"
    :modelValue="modelValue"
    :type="type ?? 'text'"
    :size="size ?? 'md'"
    :placeholder="placeholder"
    :invalid="invalid"
    :disabled="disabled"
    :readonly="readonly"
    v-bind="$attrs"
    @update:modelValue="(v: string) => emit('update:modelValue', v)"
  >
    <template v-if="$slots.prefix" #prefix><slot name="prefix" /></template>
    <template v-if="$slots.suffix" #suffix><slot name="suffix" /></template>
  </component>
</template>
