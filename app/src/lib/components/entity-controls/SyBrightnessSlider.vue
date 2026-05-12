<!--
  SyBrightnessSlider — 0-100% slider for light brightness.

  Owns its own draft value while dragging so external prop updates
  (from incoming EntityChange events) don't fight the user. Releases
  the drag flag 500ms after `change` fires — gives the daemon's echo
  back through the stream a moment to land.
-->
<script setup lang="ts">
import { ref, watch } from "vue";

const props = defineProps<{
  /** Current brightness, 0..255. */
  value: number;
  busy?: boolean;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "commit", next: number): void;
}>();

function toPercent(v: number): number { return Math.round((v / 255) * 100); }
function fromPercent(p: number): number { return Math.round((p / 100) * 255); }

const draft = ref<number>(toPercent(props.value));
const dragging = ref<boolean>(false);

watch(() => props.value, (v) => {
  if (!dragging.value) draft.value = toPercent(v);
});

function onInput(e: Event): void {
  draft.value = Number((e.target as HTMLInputElement).value);
  dragging.value = true;
}
function onChange(): void {
  emit("commit", fromPercent(draft.value));
  window.setTimeout(() => { dragging.value = false; }, 500);
}
</script>

<template>
  <div class="sy-brightness">
    <input
      type="range"
      min="0"
      max="100"
      step="1"
      :value="draft"
      :disabled="disabled || busy"
      @input="onInput"
      @change="onChange"
    />
    <span class="sy-brightness__pct">{{ draft }}%</span>
  </div>
</template>

<style scoped>
.sy-brightness { display: flex; align-items: center; gap: var(--sy-space-2); }
.sy-brightness input { flex: 1; }
.sy-brightness__pct {
  font-variant-numeric: tabular-nums;
  font-size: var(--sy-font-size-caption);
  color: var(--sy-color-fg-subtle);
  min-width: 3ch; text-align: right;
}
</style>
