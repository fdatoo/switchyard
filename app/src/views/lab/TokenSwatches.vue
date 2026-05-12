<script setup lang="ts">
import { SyText } from "@/lib";

const COLOR_TOKENS = [
  "--sy-color-bg",
  "--sy-color-surface-1",
  "--sy-color-surface-2",
  "--sy-color-surface-3",
  "--sy-color-sidebar",
  "--sy-color-line",
  "--sy-color-fg",
  "--sy-color-fg-2",
  "--sy-color-fg-3",
  "--sy-color-fg-4",
  "--sy-color-fg-5",
  "--sy-color-accent",
  "--sy-color-accent-2",
  "--sy-color-accent-soft",
  "--sy-color-good",
  "--sy-color-warn",
  "--sy-color-bad",
  "--sy-color-info",
  "--sy-color-purple",
];

const RADIUS_TOKENS = ["--sy-radius-sm", "--sy-radius", "--sy-radius-lg", "--sy-radius-xl"];
const SPACE_TOKENS  = ["--sy-space-1", "--sy-space-2", "--sy-space-3", "--sy-space-4", "--sy-space-5", "--sy-space-6"];
</script>

<template>
  <div class="swatches">
    <div class="group">
      <SyText variant="label" tone="subtle">Color</SyText>
      <div class="colorGrid">
        <div v-for="t in COLOR_TOKENS" :key="t" class="colorCell">
          <div class="colorChip" :style="{ background: `var(${t})` }" />
          <SyText variant="caption" tone="subtle" class="tokenName">{{ t.replace('--sy-color-', '') }}</SyText>
        </div>
      </div>
    </div>

    <div class="group">
      <SyText variant="label" tone="subtle">Radius</SyText>
      <div class="radiusRow">
        <div v-for="t in RADIUS_TOKENS" :key="t" class="radiusCell">
          <div class="radiusBox" :style="{ borderRadius: `var(${t})` }" />
          <SyText variant="caption" tone="subtle">{{ t.replace('--sy-radius', '').replace('-', '') || 'base' }}</SyText>
        </div>
      </div>
    </div>

    <div class="group">
      <SyText variant="label" tone="subtle">Space</SyText>
      <div class="spaceCol">
        <div v-for="t in SPACE_TOKENS" :key="t" class="spaceRow">
          <div class="spaceBar" :style="{ width: `calc(var(${t}) * 6)` }" />
          <SyText variant="caption" tone="subtle">{{ t.replace('--sy-space-', '') }}</SyText>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.swatches {
  padding: var(--sy-space-4);
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-4);
}

.group {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-2);
}

.colorGrid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(80px, 1fr));
  gap: var(--sy-space-2);
}
.colorCell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.colorChip {
  height: 40px;
  border-radius: var(--sy-radius);
  border: 1px solid var(--sy-color-line);
}
.tokenName {
  font-family: var(--sy-font-numeric);
  font-size: 0.6875rem;
}

.radiusRow {
  display: flex;
  gap: var(--sy-space-3);
  align-items: flex-end;
}
.radiusCell {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}
.radiusBox {
  width: 48px;
  height: 48px;
  background: var(--sy-color-surface-2);
  border: 1px solid var(--sy-color-line);
}

.spaceCol {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-1);
}
.spaceRow {
  display: flex;
  align-items: center;
  gap: var(--sy-space-2);
}
.spaceBar {
  height: 8px;
  background: var(--sy-color-accent);
  border-radius: 2px;
}
</style>
