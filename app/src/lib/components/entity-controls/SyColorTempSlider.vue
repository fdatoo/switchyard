<!--
  SyColorTempSlider — color temperature in mireds. Hue's typical range
  is ~153 (cool, ~6500K) to ~500 (warm, ~2000K). Same drag-throttling
  pattern as SyBrightnessSlider.
-->
<script setup lang="ts">
import { ref, watch } from "vue";

const props = defineProps<{
  value: number;
  min?: number;
  max?: number;
  busy?: boolean;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "commit", next: number): void;
}>();

const draft = ref<number>(props.value);
const dragging = ref<boolean>(false);

watch(() => props.value, (v) => {
  if (!dragging.value) draft.value = v;
});

function onInput(e: Event): void {
  draft.value = Number((e.target as HTMLInputElement).value);
  dragging.value = true;
}
function onChange(): void {
  emit("commit", draft.value);
  window.setTimeout(() => { dragging.value = false; }, 500);
}
</script>

<template>
  <div class="sy-temp">
    <input
      type="range"
      :min="min ?? 153"
      :max="max ?? 500"
      step="1"
      :value="draft"
      :disabled="disabled || busy"
      @input="onInput"
      @change="onChange"
    />
    <span class="sy-temp__val">{{ draft }} mired</span>
  </div>
</template>

<style scoped>
.sy-temp { display: flex; align-items: center; gap: var(--sy-space-2); }
.sy-temp input { flex: 1; }
.sy-temp__val {
  font-variant-numeric: tabular-nums;
  font-size: var(--sy-font-size-caption);
  color: var(--sy-color-fg-subtle);
  min-width: 8ch; text-align: right;
}
</style>
