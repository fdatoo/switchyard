<script setup lang="ts">
import { ref } from "vue";
import { SyText, SyTopBar } from "@/lib";
import type { BreadcrumbItem } from "@/lib/components/breadcrumb/SyBreadcrumb.vue";

const lastEvent = ref("");

const homeCrumbs: BreadcrumbItem[] = [{ label: "Home" }];

const settingsCrumbs: BreadcrumbItem[] = [
  { label: "Settings", to: "/settings" },
  { label: "Drivers" },
];

const deepCrumbs: BreadcrumbItem[] = [
  { label: "Devices", to: "/devices" },
  { label: "hue_main", to: "/devices/hue_main" },
  { label: "kitchen_pendant" },
];
</script>

<template>
  <div class="specimen">
    <SyText variant="caption" tone="subtle">
      Last: <SyText as="span" weight="medium">{{ lastEvent || "—" }}</SyText>
    </SyText>

    <div class="block">
      <SyText variant="label" tone="subtle">Home · daemon ok</SyText>
      <SyTopBar :crumbs="homeCrumbs" daemon-status="ok" @search="lastEvent = 'search'" />
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Settings › Drivers · reconnecting</SyText>
      <SyTopBar :crumbs="settingsCrumbs" daemon-status="reconnecting" @search="lastEvent = 'search'" />
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Deep crumbs · daemon down</SyText>
      <SyTopBar :crumbs="deepCrumbs" daemon-status="down" @search="lastEvent = 'search'" />
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-3); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
</style>
