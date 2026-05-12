<script setup lang="ts">
import { SyText } from "@/lib";

defineProps<{ title: string; caption?: string }>();
</script>

<template>
  <section class="section">
    <header class="section__head">
      <SyText as="h2" variant="title">{{ title }}</SyText>
      <SyText v-if="caption" variant="caption" tone="subtle">{{ caption }}</SyText>
    </header>
    <div class="section__body">
      <slot />
    </div>
  </section>
</template>

<style scoped>
.section {
  padding: var(--sy-space-6);
  border-bottom: 1px solid var(--sy-color-line-soft);
}
.section__head {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-1);
  margin-bottom: var(--sy-space-4);
  max-width: 720px;
}
.section__body {
  display: flex;
  flex-direction: column;
}
</style>
