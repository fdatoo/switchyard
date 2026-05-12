<!--
  SettingsStub — reusable empty-state body for unbuilt settings sections.
  Routes pass their own title/description via props.
-->
<script setup lang="ts">
import { SyText, SyEmptyState, SyIcon } from "@/lib";
import type { IconName } from "@/lib/components/icon/SyIcon.vue";

interface Props {
  title: string;
  description: string;
  icon?: IconName;
}
withDefaults(defineProps<Props>(), { icon: "settings" });
</script>

<template>
  <section class="section">
    <header class="section__head">
      <SyText as="h1" variant="display">{{ title }}</SyText>
    </header>
    <SyEmptyState
      :title="`${title} is coming soon`"
      :description="description"
    >
      <template #icon><SyIcon :name="icon" :size="28" /></template>
    </SyEmptyState>
  </section>
</template>

<style scoped>
.section { display: flex; flex-direction: column; gap: var(--sy-space-4); }
.section__head { display: flex; flex-direction: column; gap: var(--sy-space-1); }
</style>
