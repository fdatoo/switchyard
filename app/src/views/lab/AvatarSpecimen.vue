<script setup lang="ts">
import { SyText, SyAvatar } from "@/lib";
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Sizes · initials</SyText>
      <div class="row">
        <SyAvatar name="Fynn Datoo" size="sm" />
        <SyAvatar name="Fynn Datoo" size="md" />
        <SyAvatar name="Fynn Datoo" size="lg" />
        <SyAvatar name="Fynn Datoo" size="xl" />
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Initials derivation</SyText>
      <div class="row">
        <span class="item">
          <SyAvatar name="Fynn Datoo" />
          <SyText variant="caption" tone="subtle">Fynn Datoo → FD</SyText>
        </span>
        <span class="item">
          <SyAvatar name="kitchen" />
          <SyText variant="caption" tone="subtle">kitchen → K</SyText>
        </span>
        <span class="item">
          <SyAvatar name="Hue Bridge" />
          <SyText variant="caption" tone="subtle">Hue Bridge → HB</SyText>
        </span>
        <span class="item">
          <SyAvatar />
          <SyText variant="caption" tone="subtle">no name → gradient</SyText>
        </span>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Shape</SyText>
      <div class="row">
        <SyAvatar name="Fynn Datoo" shape="circle" />
        <SyAvatar name="Hue Bridge" shape="square" />
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Broken src → fallback to initials</SyText>
      <SyAvatar src="https://example.invalid/missing.png" name="Fynn Datoo" />
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">In context · user pill</SyText>
      <div class="pill">
        <SyAvatar name="Fynn Datoo" size="sm" />
        <SyText variant="caption" weight="medium">Fynn Datoo</SyText>
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.row { display: flex; flex-wrap: wrap; gap: var(--sy-space-3); align-items: center; }
.item {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.pill {
  display: inline-flex;
  align-items: center;
  gap: var(--sy-space-2);
  padding: 6px 10px 6px 6px;
  background: var(--sy-color-surface-2);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-pill);
}
</style>
