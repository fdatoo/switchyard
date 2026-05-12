<script setup lang="ts">
import { SyText, SySpinner, SyButton } from "@/lib";
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Sizes</SyText>
      <div class="row">
        <SySpinner :size="14" />
        <SySpinner :size="18" />
        <SySpinner :size="24" />
        <SySpinner :size="32" />
        <SySpinner :size="48" />
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Tones (via parent color)</SyText>
      <div class="row">
        <span style="color: var(--sy-color-fg)"><SySpinner /></span>
        <span style="color: var(--sy-color-fg-3)"><SySpinner /></span>
        <span style="color: var(--sy-color-accent)"><SySpinner /></span>
        <span style="color: var(--sy-color-good)"><SySpinner /></span>
        <span style="color: var(--sy-color-warn)"><SySpinner /></span>
        <span style="color: var(--sy-color-bad)"><SySpinner /></span>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Inside a button</SyText>
      <div class="row">
        <SyButton disabled>
          <SySpinner :size="14" />
          Saving…
        </SyButton>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">As page loader</SyText>
      <div class="loader">
        <SySpinner :size="24" />
        <SyText variant="caption" tone="subtle">Loading drivers…</SyText>
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.row { display: flex; flex-wrap: wrap; gap: var(--sy-space-3); align-items: center; }

.loader {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--sy-space-2);
  padding: var(--sy-space-5);
  background: var(--sy-color-surface-2);
  border-radius: var(--sy-radius);
  color: var(--sy-color-accent);
}
</style>
