<!--
  SyColorPicker — minimal HTML5 color input. Proto stores RGB as uint32
  0xRRGGBB; we serialize/parse against the standard "#rrggbb" hex form
  HTML5 emits.
-->
<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{
  /** Current color as 0xRRGGBB (uint32). 0 means unset / unsupported. */
  value: number;
  busy?: boolean;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "commit", rgbHex: string): void;
}>();

const hex = computed<string>(() => {
  const n = (props.value | 0) & 0xffffff;
  return "#" + n.toString(16).padStart(6, "0");
});

function onChange(e: Event): void {
  const v = (e.target as HTMLInputElement).value; // "#rrggbb"
  emit("commit", v.startsWith("#") ? v.slice(1) : v);
}
</script>

<template>
  <input
    type="color"
    :value="hex"
    :disabled="disabled || busy"
    @change="onChange"
  />
</template>

<style scoped>
input[type="color"] {
  width: 36px; height: 28px;
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-sm);
  background: transparent;
  padding: 2px;
}
</style>
