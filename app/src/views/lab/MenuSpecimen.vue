<script setup lang="ts">
import { ref } from "vue";
import { SyText, SyMenu, SyButton, SyIcon, SyAvatar } from "@/lib";
import type { MenuItem } from "@/lib/components/menu/types";

const lastSelected = ref<string>("");

const overflow: MenuItem[] = [
  { type: "item", id: "edit", label: "Edit", icon: "settings", shortcut: "⌘E" },
  { type: "item", id: "duplicate", label: "Duplicate", icon: "plus", shortcut: "⌘D" },
  { type: "separator" },
  { type: "item", id: "restart", label: "Restart driver", icon: "power" },
  { type: "separator" },
  { type: "item", id: "delete", label: "Delete", icon: "close", intent: "danger", shortcut: "⌘⌫" },
];

const filter: MenuItem[] = [
  { type: "header", label: "Sort by" },
  { type: "item", id: "name", label: "Name (A→Z)" },
  { type: "item", id: "recent", label: "Most recent" },
  { type: "item", id: "active", label: "Most active" },
  { type: "separator" },
  { type: "header", label: "Filter" },
  { type: "item", id: "running", label: "Running only", icon: "check" },
  { type: "item", id: "all", label: "All states" },
];

const userMenu: MenuItem[] = [
  { type: "header", label: "Fynn Datoo" },
  { type: "item", id: "account", label: "Account settings", icon: "settings" },
  { type: "item", id: "theme", label: "Theme & language", icon: "sparkle" },
  { type: "separator" },
  { type: "item", id: "signout", label: "Sign out", icon: "close" },
];

const disabledDemo: MenuItem[] = [
  { type: "item", id: "a", label: "Available" },
  { type: "item", id: "b", label: "Coming soon", disabled: true },
  { type: "item", id: "c", label: "Available too" },
];
</script>

<template>
  <div class="specimen">
    <SyText variant="caption" tone="subtle">
      Selected: <SyText as="span" weight="medium">{{ lastSelected || "—" }}</SyText>
    </SyText>

    <div class="block">
      <SyText variant="label" tone="subtle">Overflow menu (canonical pattern)</SyText>
      <SyMenu :items="overflow" @select="lastSelected = $event">
        <template #trigger="{ isOpen }">
          <SyButton intent="ghost" size="sm" :aria-expanded="isOpen">More ▾</SyButton>
        </template>
      </SyMenu>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">With headers + groups</SyText>
      <SyMenu :items="filter" @select="lastSelected = $event">
        <template #trigger>
          <SyButton intent="secondary" size="sm">
            <SyIcon name="settings" :size="14" /> Filter & sort
          </SyButton>
        </template>
      </SyMenu>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">User pill menu</SyText>
      <SyMenu :items="userMenu" @select="lastSelected = $event">
        <template #trigger>
          <button class="userpill" type="button">
            <SyAvatar name="Fynn Datoo" size="sm" />
            <span>Fynn Datoo</span>
          </button>
        </template>
      </SyMenu>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Disabled items</SyText>
      <SyMenu :items="disabledDemo" @select="lastSelected = $event">
        <template #trigger>
          <SyButton intent="ghost" size="sm">Open</SyButton>
        </template>
      </SyMenu>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-3); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }

.userpill {
  display: inline-flex;
  align-items: center;
  gap: var(--sy-space-2);
  padding: 4px 10px 4px 4px;
  background: var(--sy-color-surface-2);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-pill);
  color: var(--sy-color-fg);
  font-family: var(--sy-font-body);
  font-size: 0.8125rem;
  font-weight: 500;
  cursor: pointer;
  transition: background var(--sy-motion-fast);
}
.userpill:hover {
  background: var(--sy-color-surface-3);
}
</style>
