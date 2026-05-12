<!--
  StarlarkEditorSection — /settings/starlark. Same shell as Pkl
  but bottom slot hosts SyTestPanel that targets the currently-
  open script.
-->
<script setup lang="ts">
import { SyText } from "@/lib";
import SyCodeEditorPanel from "@/lib/components/code-editor-panel/SyCodeEditorPanel.vue";
import SyTestPanel from "@/lib/components/code-editor-panel/SyTestPanel.vue";
</script>

<template>
  <div class="page">
    <header class="page__head">
      <SyText as="h1" variant="display">Starlark</SyText>
      <SyText variant="body" tone="subtle">
        Edit automation handlers. Run tests via the panel below.
      </SyText>
    </header>
    <div class="page__panel">
      <SyCodeEditorPanel kind="starlark">
        <template #bottom="{ selectedPath }">
          <SyTestPanel :path="selectedPath" />
        </template>
      </SyCodeEditorPanel>
    </div>
  </div>
</template>

<style scoped>
.page {
  padding: var(--sy-space-5) var(--sy-space-6);
  display: flex; flex-direction: column; gap: var(--sy-space-4);
  height: 100%;
}
.page__head { display: flex; flex-direction: column; gap: var(--sy-space-1); }
.page__panel { flex: 1; min-height: 0; border: 1px solid var(--sy-color-line); border-radius: var(--sy-radius-md); overflow: hidden; }
</style>
