<script setup lang="ts">
import { SyText, SySurface } from "@/lib";
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Elevations</SyText>
      <div class="grid">
        <SySurface elevation="flat" padding="md">
          <SyText variant="caption" tone="subtle">flat</SyText>
          <SyText variant="body" weight="medium">No shadow</SyText>
        </SySurface>
        <SySurface elevation="raised" padding="md">
          <SyText variant="caption" tone="subtle">raised</SyText>
          <SyText variant="body" weight="medium">Default card</SyText>
        </SySurface>
        <SySurface elevation="elevated" padding="md">
          <SyText variant="caption" tone="subtle">elevated</SyText>
          <SyText variant="body" weight="medium">Floating</SyText>
        </SySurface>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Padding</SyText>
      <div class="grid grid--tight">
        <SySurface padding="none">
          <div class="filler">none</div>
        </SySurface>
        <SySurface padding="sm">
          <SyText variant="caption">sm</SyText>
        </SySurface>
        <SySurface padding="md">
          <SyText variant="caption">md</SyText>
        </SySurface>
        <SySurface padding="lg">
          <SyText variant="caption">lg</SyText>
        </SySurface>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Interactive</SyText>
      <SySurface interactive padding="md">
        <SyText variant="body" weight="medium">Hover or focus me</SyText>
        <SyText variant="caption" tone="subtle">Border + shadow lift on hover. Cursor pointer.</SyText>
      </SySurface>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Nested · surface tiers</SyText>
      <SySurface padding="md">
        <SyText variant="caption" tone="subtle">surface-1</SyText>
        <div class="tier2">
          <SyText variant="caption" tone="subtle">surface-2 (manual)</SyText>
          <div class="tier3">
            <SyText variant="caption" tone="subtle">surface-3 (manual)</SyText>
          </div>
        </div>
      </SySurface>
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
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--sy-space-2);
}
.grid--tight { grid-template-columns: repeat(4, minmax(0, 1fr)); }

.filler {
  padding: var(--sy-space-1) var(--sy-space-2);
  background: var(--sy-color-surface-2);
  border-radius: var(--sy-radius-sm);
  font-size: 0.75rem;
  color: var(--sy-color-fg-3);
  text-align: center;
}

.tier2 {
  margin-top: var(--sy-space-2);
  padding: var(--sy-space-3);
  background: var(--sy-color-surface-2);
  border-radius: var(--sy-radius);
}
.tier3 {
  margin-top: var(--sy-space-2);
  padding: var(--sy-space-3);
  background: var(--sy-color-surface-3);
  border-radius: var(--sy-radius-sm);
}
</style>
