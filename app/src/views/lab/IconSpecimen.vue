<script setup lang="ts">
import { SyText, SyIcon } from "@/lib";
import type { IconName } from "@/lib/components/icon/SyIcon.vue";

const NAV: IconName[] = ["home", "rooms", "activity", "automations", "devices", "settings", "pages", "displays"];
const UI: IconName[] = ["search", "plus", "check", "close", "chevron-right", "chevron-down"];
const DOMAIN: IconName[] = ["power", "bulb", "thermometer", "plugin", "sparkle"];
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Navigation</SyText>
      <div class="grid">
        <div v-for="n in NAV" :key="n" class="cell">
          <SyIcon :name="n" :size="20" />
          <SyText variant="caption" tone="subtle">{{ n }}</SyText>
        </div>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">UI</SyText>
      <div class="grid">
        <div v-for="n in UI" :key="n" class="cell">
          <SyIcon :name="n" :size="20" />
          <SyText variant="caption" tone="subtle">{{ n }}</SyText>
        </div>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Domain</SyText>
      <div class="grid">
        <div v-for="n in DOMAIN" :key="n" class="cell">
          <SyIcon :name="n" :size="20" />
          <SyText variant="caption" tone="subtle">{{ n }}</SyText>
        </div>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Sizes &amp; tones</SyText>
      <div class="row">
        <SyIcon name="bulb" :size="14" />
        <SyIcon name="bulb" :size="18" />
        <SyIcon name="bulb" :size="24" />
        <SyIcon name="bulb" :size="32" />
      </div>
      <div class="row">
        <span style="color: var(--sy-color-fg)"><SyIcon name="sparkle" :size="20" /></span>
        <span style="color: var(--sy-color-fg-3)"><SyIcon name="sparkle" :size="20" /></span>
        <span style="color: var(--sy-color-accent)"><SyIcon name="sparkle" :size="20" /></span>
        <span style="color: var(--sy-color-good)"><SyIcon name="sparkle" :size="20" /></span>
        <span style="color: var(--sy-color-warn)"><SyIcon name="sparkle" :size="20" /></span>
        <span style="color: var(--sy-color-bad)"><SyIcon name="sparkle" :size="20" /></span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(72px, 1fr));
  gap: var(--sy-space-2);
}
.cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: var(--sy-space-2);
  background: var(--sy-color-surface-2);
  border-radius: var(--sy-radius-sm);
  color: var(--sy-color-fg-2);
}
.row {
  display: flex;
  align-items: center;
  gap: var(--sy-space-3);
  flex-wrap: wrap;
}
</style>
