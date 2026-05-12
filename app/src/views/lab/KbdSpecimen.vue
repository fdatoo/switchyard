<script setup lang="ts">
import { SyText, SyKbd } from "@/lib";
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Single keys</SyText>
      <div class="row">
        <SyKbd>⌘</SyKbd>
        <SyKbd>⇧</SyKbd>
        <SyKbd>⌥</SyKbd>
        <SyKbd>⌃</SyKbd>
        <SyKbd>K</SyKbd>
        <SyKbd>Esc</SyKbd>
        <SyKbd>↵</SyKbd>
        <SyKbd>Tab</SyKbd>
        <SyKbd>↑</SyKbd>
        <SyKbd>↓</SyKbd>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Combos · single chip</SyText>
      <div class="row">
        <SyKbd>⌘K</SyKbd>
        <SyKbd>⌘⇧P</SyKbd>
        <SyKbd>⌘,</SyKbd>
        <SyKbd>⌘1</SyKbd>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Combos · per-key chips</SyText>
      <div class="row">
        <span class="combo"><SyKbd>⌘</SyKbd>+<SyKbd>K</SyKbd></span>
        <span class="combo"><SyKbd>⌘</SyKbd>+<SyKbd>⇧</SyKbd>+<SyKbd>P</SyKbd></span>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Inline in text</SyText>
      <SyText variant="body">
        Press <SyKbd>⌘K</SyKbd> to open the command palette, or
        <SyKbd>?</SyKbd> for keyboard shortcut help.
      </SyText>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Sizes</SyText>
      <div class="row">
        <SyKbd size="sm">⌘K</SyKbd>
        <SyKbd size="md">⌘K</SyKbd>
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.row { display: flex; flex-wrap: wrap; gap: var(--sy-space-2); align-items: center; }
.combo {
  display: inline-flex;
  gap: 4px;
  align-items: center;
  color: var(--sy-color-fg-4);
  font-size: 0.6875rem;
}
</style>
