<script setup lang="ts">
import { SyText } from "@/lib";
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText as="h1" variant="display">Display</SyText>
      <SyText as="h2" variant="title">Title</SyText>
      <SyText as="h3" variant="subtitle">Subtitle</SyText>
      <SyText as="p"  variant="body">
        Body text. The cadence of the room is gentle but deliberate.
      </SyText>
      <SyText variant="caption" tone="subtle">Caption · 8 minutes ago</SyText>
      <SyText variant="label" tone="subtle">Label / section heading</SyText>
      <SyText variant="numeric">0123.456 / 2 → 4</SyText>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Tones</SyText>
      <div class="tones">
        <SyText variant="body" tone="default">default</SyText>
        <SyText variant="body" tone="muted">muted</SyText>
        <SyText variant="body" tone="subtle">subtle</SyText>
        <SyText variant="body" tone="accent">accent</SyText>
        <SyText variant="body" tone="good">good</SyText>
        <SyText variant="body" tone="warn">warn</SyText>
        <SyText variant="body" tone="bad">bad</SyText>
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
.tones {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sy-space-3);
}
</style>
