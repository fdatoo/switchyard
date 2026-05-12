<script setup lang="ts">
import { SyText, SyButton } from "@/lib";
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Intents · size md</SyText>
      <div class="row">
        <SyButton intent="primary">Primary</SyButton>
        <SyButton intent="secondary">Secondary</SyButton>
        <SyButton intent="ghost">Ghost</SyButton>
        <SyButton intent="danger">Danger</SyButton>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Sizes · primary</SyText>
      <div class="row">
        <SyButton intent="primary" size="sm">Small</SyButton>
        <SyButton intent="primary" size="md">Medium</SyButton>
        <SyButton intent="primary" size="lg">Large</SyButton>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Disabled</SyText>
      <div class="row">
        <SyButton intent="primary" disabled>Primary</SyButton>
        <SyButton intent="secondary" disabled>Secondary</SyButton>
        <SyButton intent="ghost" disabled>Ghost</SyButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen {
  padding: var(--sy-space-4);
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-4);
}
.block {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-2);
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sy-space-3);
  align-items: center;
}
</style>
