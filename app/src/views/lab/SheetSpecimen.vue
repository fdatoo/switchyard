<script setup lang="ts">
import { ref } from "vue";
import { SyText, SyButton, SySheet, SyListRow, SyBadge, SyDot, SyIcon } from "@/lib";

const right = ref(false);
const bottom = ref(false);
const center = ref(false);
const left = ref(false);
const top = ref(false);
</script>

<template>
  <div class="specimen">
    <SyText variant="caption" tone="subtle">
      Sheets teleport to body and render at viewport scale — use the global theme switcher above to see them per language.
    </SyText>

    <div class="block">
      <SyText variant="label" tone="subtle">Sides</SyText>
      <div class="row">
        <SyButton intent="primary" size="sm" @click="right = true">Right rail</SyButton>
        <SyButton intent="secondary" size="sm" @click="bottom = true">Bottom sheet</SyButton>
        <SyButton intent="secondary" size="sm" @click="center = true">Center modal</SyButton>
        <SyButton intent="ghost" size="sm" @click="left = true">Left</SyButton>
        <SyButton intent="ghost" size="sm" @click="top = true">Top</SyButton>
      </div>
    </div>

    <!-- Right rail: canonical detail-rail use case. Header + scrollable body + footer. -->
    <SySheet v-model="right" side="right" title="hue_main">
      <div class="detail">
        <div class="kv">
          <SyText variant="caption" tone="subtle">Pack</SyText>
          <SyText variant="numeric">switchyard/hue@2.4.1</SyText>
        </div>
        <div class="kv">
          <SyText variant="caption" tone="subtle">State</SyText>
          <SyBadge intent="good" dot pulse="slow">Running</SyBadge>
        </div>
        <div class="kv">
          <SyText variant="caption" tone="subtle">Entities</SyText>
          <SyText variant="body">12</SyText>
        </div>
        <SyText variant="label" tone="subtle">Recent events</SyText>
        <SyListRow density="compact" v-for="e in 4" :key="e">
          <template #leading><SyDot intent="info" /></template>
          <SyText variant="caption">light.kitchen_pendant turned on</SyText>
          <template #trailing>
            <SyText variant="caption" tone="subtle">12:42</SyText>
          </template>
        </SyListRow>
      </div>
      <template #footer>
        <SyButton intent="ghost" size="sm" @click="right = false">Close</SyButton>
        <SyButton intent="secondary" size="sm">Restart</SyButton>
        <SyButton intent="primary" size="sm">View entities</SyButton>
      </template>
    </SySheet>

    <SySheet v-model="bottom" side="bottom" title="Quick action">
      <SyText variant="body">Bottom sheet — used on mobile for the same detail content the desktop renders as a right rail.</SyText>
      <template #footer>
        <SyButton intent="primary" size="sm" @click="bottom = false">Done</SyButton>
      </template>
    </SySheet>

    <SySheet v-model="center" side="center" size="sm" title="Confirm delete">
      <SyText variant="body">
        This will permanently remove <SyText as="span" weight="medium">automation.sunset_lights</SyText> from your Pkl config. This action can't be undone.
      </SyText>
      <template #footer>
        <SyButton intent="ghost" size="sm" @click="center = false">Cancel</SyButton>
        <SyButton intent="danger" size="sm" @click="center = false">Delete</SyButton>
      </template>
    </SySheet>

    <SySheet v-model="left" side="left" title="Filters">
      <SyText variant="body">Left rail.</SyText>
    </SySheet>

    <SySheet v-model="top" side="top" title="Reconnecting…">
      <SyText variant="body">Top sheet — useful for system-level notifications.</SyText>
    </SySheet>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-3); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.row { display: flex; flex-wrap: wrap; gap: var(--sy-space-2); align-items: center; }

.detail {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-3);
}
.kv {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
</style>
