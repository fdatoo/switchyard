<script setup lang="ts">
import { SyText, SyStatusBar, SyDot, SyIcon, SyKbd } from "@/lib";
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Pkl editor</SyText>
      <div class="editor-frame">
        <div class="editor-body">
          <SyText variant="caption" tone="subtle">— code area —</SyText>
        </div>
        <SyStatusBar>
          <template #left>
            <span class="item">
              <SyIcon name="check" :size="11" /> main.pkl
            </span>
            <span class="item sep">modified</span>
          </template>
          <template #right>
            <span class="item mono">Pkl 0.27.1</span>
            <span class="item mono sep">Ln 42, Col 18</span>
            <span class="item sep">UTF-8</span>
          </template>
        </SyStatusBar>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Daemon status (developer page)</SyText>
      <div class="editor-frame">
        <div class="editor-body short">
          <SyText variant="caption" tone="subtle">— page content —</SyText>
        </div>
        <SyStatusBar>
          <template #left>
            <span class="item">
              <SyDot intent="good" pulse="slow" size="sm" /> connected
            </span>
            <span class="item mono sep">12ms p50 · 38ms p99</span>
          </template>
          <template #center>
            <span class="item mono">build a1b2c3d</span>
          </template>
          <template #right>
            <span class="item">
              <SyKbd>⌘K</SyKbd> palette
            </span>
            <span class="item sep">
              <SyKbd>?</SyKbd> help
            </span>
          </template>
        </SyStatusBar>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Muted variant</SyText>
      <div class="editor-frame">
        <div class="editor-body short">
          <SyText variant="caption" tone="subtle">— ambient surface —</SyText>
        </div>
        <SyStatusBar muted>
          <template #left>
            <span class="item">
              <SyDot intent="warn" pulse="fast" size="sm" /> reconnecting…
            </span>
          </template>
        </SyStatusBar>
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }

.editor-frame {
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius);
  overflow: hidden;
  background: var(--sy-color-surface-1);
}
.editor-body {
  padding: var(--sy-space-5) var(--sy-space-4);
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 80px;
}
.editor-body.short { min-height: 40px; padding: var(--sy-space-3) var(--sy-space-4); }

.item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.mono { font-family: var(--sy-font-numeric); font-feature-settings: var(--sy-numeric-feature); }
.sep { padding-left: var(--sy-space-3); border-left: 1px solid var(--sy-color-line); }
</style>
