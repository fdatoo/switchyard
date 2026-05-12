<script setup lang="ts">
import { SyText, SyTooltip, SyButton, SyIcon } from "@/lib";
</script>

<template>
  <div class="specimen">
    <SyText variant="caption" tone="subtle">
      Tooltips teleport to body and use the document-level theme — hover any trigger to verify.
    </SyText>

    <div class="block">
      <SyText variant="label" tone="subtle">Sides (preferred; flips on overflow)</SyText>
      <div class="row row--padded">
        <SyTooltip content="Above" side="top">
          <SyButton intent="secondary" size="sm">Top</SyButton>
        </SyTooltip>
        <SyTooltip content="Below" side="bottom">
          <SyButton intent="secondary" size="sm">Bottom</SyButton>
        </SyTooltip>
        <SyTooltip content="To the left" side="left">
          <SyButton intent="secondary" size="sm">Left</SyButton>
        </SyTooltip>
        <SyTooltip content="To the right" side="right">
          <SyButton intent="secondary" size="sm">Right</SyButton>
        </SyTooltip>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">On an icon-only button</SyText>
      <div class="row">
        <SyTooltip content="Open command palette · ⌘K">
          <button class="iconbtn" type="button" aria-label="Search">
            <SyIcon name="search" :size="18" />
          </button>
        </SyTooltip>
        <SyTooltip content="New page · ⌘N">
          <button class="iconbtn" type="button" aria-label="New page">
            <SyIcon name="plus" :size="18" />
          </button>
        </SyTooltip>
        <SyTooltip content="Settings · ⌘,">
          <button class="iconbtn" type="button" aria-label="Settings">
            <SyIcon name="settings" :size="18" />
          </button>
        </SyTooltip>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Rich content via slot</SyText>
      <SyTooltip>
        <SyButton intent="ghost" size="sm">Hover for details</SyButton>
        <template #content>
          Press <kbd>⌘K</kbd> anywhere to open the command palette.
        </template>
      </SyTooltip>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.row { display: flex; flex-wrap: wrap; gap: var(--sy-space-3); align-items: center; }
.row--padded { padding: var(--sy-space-4) 0; }

.iconbtn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  background: var(--sy-color-surface-1);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius);
  color: var(--sy-color-fg-2);
  cursor: pointer;
  transition: background var(--sy-motion-fast), color var(--sy-motion-fast);
}
.iconbtn:hover {
  background: var(--sy-color-surface-2);
  color: var(--sy-color-fg);
}
</style>
