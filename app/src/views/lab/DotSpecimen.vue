<script setup lang="ts">
import { SyText, SyDot } from "@/lib";
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Intents · md</SyText>
      <div class="row">
        <span class="item"><SyDot intent="neutral" /><SyText variant="caption">neutral</SyText></span>
        <span class="item"><SyDot intent="accent" /><SyText variant="caption">accent</SyText></span>
        <span class="item"><SyDot intent="good" /><SyText variant="caption">good</SyText></span>
        <span class="item"><SyDot intent="warn" /><SyText variant="caption">warn</SyText></span>
        <span class="item"><SyDot intent="bad" /><SyText variant="caption">bad</SyText></span>
        <span class="item"><SyDot intent="info" /><SyText variant="caption">info</SyText></span>
        <span class="item"><SyDot intent="purple" /><SyText variant="caption">purple</SyText></span>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Sizes</SyText>
      <div class="row">
        <SyDot intent="accent" size="sm" />
        <SyDot intent="accent" size="md" />
        <SyDot intent="accent" size="lg" />
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Pulse</SyText>
      <div class="row">
        <span class="item"><SyDot intent="good" pulse="slow" /><SyText variant="caption">slow · alive</SyText></span>
        <span class="item"><SyDot intent="warn" pulse="fast" /><SyText variant="caption">fast · in-flight</SyText></span>
        <span class="item"><SyDot intent="bad" /><SyText variant="caption">off · static</SyText></span>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">In context · driver list rows</SyText>
      <div class="rows">
        <div class="rrow"><SyDot intent="good" pulse="slow" /><SyText>hue_main</SyText><SyText variant="caption" tone="subtle">running</SyText></div>
        <div class="rrow"><SyDot intent="warn" pulse="fast" /><SyText>matter_bridge</SyText><SyText variant="caption" tone="subtle">reconnecting</SyText></div>
        <div class="rrow"><SyDot intent="neutral" /><SyText>shelly_legacy</SyText><SyText variant="caption" tone="subtle">idle</SyText></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.row { display: flex; flex-wrap: wrap; gap: var(--sy-space-3); align-items: center; }
.item { display: inline-flex; align-items: center; gap: var(--sy-space-1); }

.rows { display: flex; flex-direction: column; gap: 1px; }
.rrow {
  display: grid;
  grid-template-columns: max-content 1fr max-content;
  gap: var(--sy-space-2);
  align-items: center;
  padding: var(--sy-space-2) var(--sy-space-3);
  background: var(--sy-color-surface-2);
  border-radius: var(--sy-radius-sm);
}
</style>
