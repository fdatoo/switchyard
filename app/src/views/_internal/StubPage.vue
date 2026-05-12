<!--
  StubPage — internal placeholder for views that aren't built yet.

  Lives under `views/_internal/` to signal it's not a route, not a lib
  primitive — just a temporary scaffold for routes we've reserved but
  haven't implemented. Delete the file when the last consumer is gone.

  Standardizes the look so all unbuilt pages render the same way and the
  shell layout stays sane while we work through the routes one by one.
-->
<script setup lang="ts">
import { SyText, SyEmptyState, SySurface, SyIcon } from "@/lib";
import type { IconName } from "@/lib/components/icon/SyIcon.vue";

defineProps<{
  title: string;
  subtitle: string;
  icon: IconName;
  description: string;
}>();
</script>

<template>
  <div class="stub">
    <header class="stub__head">
      <SyText as="h1" variant="display">{{ title }}</SyText>
      <SyText variant="body" tone="subtle">{{ subtitle }}</SyText>
    </header>
    <SySurface padding="none">
      <SyEmptyState title="Not built yet" :description="description">
        <template #icon><SyIcon :name="icon" :size="28" /></template>
      </SyEmptyState>
    </SySurface>
  </div>
</template>

<style scoped>
.stub {
  padding: var(--sy-space-5) var(--sy-space-6);
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-4);
  max-width: 1080px;
}
.stub__head {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-1);
}
</style>
