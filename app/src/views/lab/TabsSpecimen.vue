<script setup lang="ts">
import { ref } from "vue";
import { SyText, SyTabs, SySurface } from "@/lib";
import type { TabDef } from "@/lib/components/tabs/types";

const activityTabs: TabDef[] = [
  { id: "stories", label: "Stories", badge: { count: 12, intent: "accent" } },
  { id: "all", label: "All events", badge: { count: 1248 } },
  { id: "saved", label: "Saved", badge: { count: 3 } },
];
const activityActive = ref<string>("stories");

const settingsTabs: TabDef[] = [
  { id: "account", label: "Account" },
  { id: "drivers", label: "Drivers", icon: "plugin", badge: { count: 3, intent: "warn" } },
  { id: "pkl", label: "Pkl config" },
  { id: "widgets", label: "Widget packs", disabled: true },
];
const settingsActive = ref<string>("drivers");

const sizes: TabDef[] = [
  { id: "a", label: "First" },
  { id: "b", label: "Second" },
  { id: "c", label: "Third" },
];
const sm = ref("a");
const md = ref("a");
const lg = ref("a");
const fill = ref("a");
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Activity tabs · with counts</SyText>
      <SyTabs v-model="activityActive" :tabs="activityTabs" />
      <SySurface padding="md">
        <SyText variant="caption" tone="subtle">Active: {{ activityActive }}</SyText>
      </SySurface>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Settings · icons + disabled</SyText>
      <SyTabs v-model="settingsActive" :tabs="settingsTabs" />
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Sizes</SyText>
      <SyTabs v-model="sm" :tabs="sizes" size="sm" />
      <SyTabs v-model="md" :tabs="sizes" size="md" />
      <SyTabs v-model="lg" :tabs="sizes" size="lg" />
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Fill width</SyText>
      <SyTabs v-model="fill" :tabs="sizes" fill />
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
</style>
