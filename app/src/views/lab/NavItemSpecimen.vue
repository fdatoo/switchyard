<script setup lang="ts">
import { ref } from "vue";
import { SyText, SyNavItem } from "@/lib";
import type { IconName } from "@/lib/components/icon/SyIcon.vue";

interface NavDef {
  id: string;
  icon: IconName;
  label: string;
  shortcut?: string;
  badge?: { count: number; intent?: "neutral" | "accent" | "good" | "warn" | "bad" };
}

const PRIMARY: NavDef[] = [
  { id: "home", icon: "home", label: "Home", shortcut: "⌘1" },
  { id: "rooms", icon: "rooms", label: "Rooms", shortcut: "⌘2" },
  { id: "activity", icon: "activity", label: "Activity", shortcut: "⌘3", badge: { count: 12, intent: "accent" } },
  { id: "automations", icon: "automations", label: "Automations", shortcut: "⌘4" },
  { id: "devices", icon: "devices", label: "Devices", shortcut: "⌘5", badge: { count: 3, intent: "warn" } },
  { id: "settings", icon: "settings", label: "Settings", shortcut: "⌘," },
];

const active = ref<string>("activity");
</script>

<template>
  <div class="specimen">
    <SyText variant="caption" tone="subtle">Shortcut chips appear in developer language only.</SyText>
    <div class="sidebar">
      <div class="sidebar__brand">
        <span class="brand-square" />
        <SyText variant="body" weight="semibold">Switchyard</SyText>
      </div>
      <div class="sidebar__nav">
        <SyNavItem
          v-for="item in PRIMARY"
          :key="item.id"
          :icon="item.icon"
          :label="item.label"
          :shortcut="item.shortcut"
          :badge="item.badge"
          :active="active === item.id"
          as="button"
          @click="active = item.id"
        />
      </div>
      <div class="sidebar__section">
        <SyText variant="label" tone="subtle">Pages</SyText>
        <SyText variant="caption" tone="subtle" style="font-style: italic;">No custom pages yet.</SyText>
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-3); }
.sidebar {
  background: var(--sy-color-sidebar);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-lg);
  padding: var(--sy-space-3) 10px;
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-3);
}
.sidebar__brand {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 4px 10px;
}
.brand-square {
  width: 22px;
  height: 22px;
  border-radius: 7px;
  background: linear-gradient(135deg, var(--sy-color-accent), var(--sy-color-accent-2));
  box-shadow: var(--sy-shadow);
  flex-shrink: 0;
}
.sidebar__nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.sidebar__section {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: var(--sy-space-2) 10px 0;
  border-top: 1px solid var(--sy-color-line-soft);
  padding-top: var(--sy-space-3);
}
</style>
