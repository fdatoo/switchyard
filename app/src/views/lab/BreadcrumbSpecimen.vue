<script setup lang="ts">
import { SyText, SyBreadcrumb } from "@/lib";
import type { BreadcrumbItem } from "@/lib/components/breadcrumb/SyBreadcrumb.vue";

const simple: BreadcrumbItem[] = [{ label: "Home" }];

const twoLevel: BreadcrumbItem[] = [
  { label: "Settings", to: "/settings" },
  { label: "Drivers" },
];

const deep: BreadcrumbItem[] = [
  { label: "Devices", to: "/devices" },
  { label: "hue_main", to: "/devices/hue_main" },
  { label: "kitchen_pendant" },
];

const long: BreadcrumbItem[] = [
  { label: "Settings", to: "/settings" },
  { label: "Pkl config", to: "/settings/pkl-config" },
  { label: "internal/config/pkl/switchyard/automations/sunset_lights.pkl" },
];
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Single (page root)</SyText>
      <SyBreadcrumb :items="simple" />
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Two-level</SyText>
      <SyBreadcrumb :items="twoLevel" />
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Three-level</SyText>
      <SyBreadcrumb :items="deep" />
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Long trailing crumb (truncates)</SyText>
      <SyBreadcrumb :items="long" />
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">In a topbar context</SyText>
      <div class="topbar">
        <SyBreadcrumb :items="deep" />
        <span class="topbar__right">
          <SyText variant="caption" tone="subtle">⌘K</SyText>
        </span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--sy-space-3);
  padding: var(--sy-space-3) var(--sy-space-4);
  background: var(--sy-color-surface-1);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius);
}
.topbar__right { color: var(--sy-color-fg-3); }
</style>
