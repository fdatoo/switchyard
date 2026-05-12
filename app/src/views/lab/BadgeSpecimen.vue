<script setup lang="ts">
import { SyText, SyBadge } from "@/lib";
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Soft · all intents</SyText>
      <div class="row">
        <SyBadge intent="neutral">Neutral</SyBadge>
        <SyBadge intent="accent">Accent</SyBadge>
        <SyBadge intent="good">Good</SyBadge>
        <SyBadge intent="warn">Warn</SyBadge>
        <SyBadge intent="bad">Bad</SyBadge>
        <SyBadge intent="info">Info</SyBadge>
        <SyBadge intent="purple">Purple</SyBadge>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Solid</SyText>
      <div class="row">
        <SyBadge appearance="solid" intent="accent">Accent</SyBadge>
        <SyBadge appearance="solid" intent="good">Good</SyBadge>
        <SyBadge appearance="solid" intent="warn">Warn</SyBadge>
        <SyBadge appearance="solid" intent="bad">Bad</SyBadge>
        <SyBadge appearance="solid" intent="info">Info</SyBadge>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Outline</SyText>
      <div class="row">
        <SyBadge appearance="outline" intent="neutral">Neutral</SyBadge>
        <SyBadge appearance="outline" intent="accent">Accent</SyBadge>
        <SyBadge appearance="outline" intent="good">Good</SyBadge>
        <SyBadge appearance="outline" intent="bad">Bad</SyBadge>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">With dot · real statuses</SyText>
      <div class="row">
        <SyBadge intent="good" dot pulse="slow">Running</SyBadge>
        <SyBadge intent="warn" dot pulse="fast">Reconnecting</SyBadge>
        <SyBadge intent="bad" dot>Failed</SyBadge>
        <SyBadge intent="neutral" dot>Idle</SyBadge>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Sizes</SyText>
      <div class="row">
        <SyBadge size="sm" intent="accent">Small</SyBadge>
        <SyBadge size="md" intent="accent">Medium</SyBadge>
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.row { display: flex; flex-wrap: wrap; gap: var(--sy-space-2); align-items: center; }
</style>
