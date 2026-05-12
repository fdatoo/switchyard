<!--
  SySearchInput — search-shaped wrapper around SyInput.

  Adds a search-icon prefix and a clear-button (×) suffix that appears only
  when the field has content. v-model passthrough; everything else flows
  through SyInput's variant system, so search inputs render correctly in
  every language without their own variants.

  Used in: filter toolbars (above tables/lists), the command palette,
  sidebar searches, anywhere "search through something" is the affordance.
-->
<script setup lang="ts">
import SyInput from "@/lib/components/input/SyInput.vue";
import SyIcon from "@/lib/components/icon/SyIcon.vue";
import type { InputSize } from "@/lib/components/input/types";

withDefaults(
  defineProps<{
    modelValue?: string;
    placeholder?: string;
    size?: InputSize;
    disabled?: boolean;
  }>(),
  { placeholder: "Search…", size: "md" },
);

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();
</script>

<template>
  <SyInput
    type="search"
    :modelValue="modelValue"
    :placeholder="placeholder"
    :size="size"
    :disabled="disabled"
    @update:modelValue="(v: string) => emit('update:modelValue', v)"
  >
    <template #prefix>
      <SyIcon name="search" :size="14" />
    </template>
    <template v-if="modelValue" #suffix>
      <button
        type="button"
        class="sy-search__clear"
        aria-label="Clear search"
        @click="emit('update:modelValue', '')"
      >
        <SyIcon name="close" :size="12" />
      </button>
    </template>
  </SyInput>
</template>

<style scoped>
.sy-search__clear {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  background: transparent;
  border: 0;
  border-radius: var(--sy-radius-pill);
  color: var(--sy-color-fg-4);
  cursor: pointer;
  transition: background var(--sy-motion-fast), color var(--sy-motion-fast);
}
.sy-search__clear:hover {
  background: var(--sy-color-surface-3);
  color: var(--sy-color-fg);
}
</style>
