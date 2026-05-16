<!--
  PklEditorSection — /settings/pkl. Wraps SyCodeEditorPanel with
  kind="pkl". No bottom slot in v1 (validation diagnostics
  live alongside the save flow as toasts/inline errors).
-->
<script setup lang="ts">
import { SyText } from "@/lib";
import SyCodeEditorPanel from "@/lib/components/code-editor-panel/SyCodeEditorPanel.vue";
</script>

<template>
  <div class="page">
    <header class="page__head">
      <SyText as="h1" variant="display">Pkl config</SyText>
      <SyText variant="body" tone="subtle">
        Edit declarative config. Saves apply on the daemon's next reload.
      </SyText>
    </header>
    <div class="page__panel">
      <SyCodeEditorPanel kind="pkl" />
    </div>
  </div>
</template>

<style scoped>
.page {
  padding: var(--sy-space-2) 0 0;
  display: flex; flex-direction: column; gap: var(--sy-space-4);
  height: 100%;
  min-height: 0;
}
.page__head { display: flex; flex-direction: column; gap: var(--sy-space-1); }
.page__panel { flex: 1; min-height: 0; border: 1px solid var(--sy-color-line); border-radius: var(--sy-radius-md); overflow: hidden; }
</style>
