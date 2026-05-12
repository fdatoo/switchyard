<!--
  SyCodeEditor — a thin Monaco wrapper.

  Props:
    modelValue   string  — editor text (v-model compatible)
    language     "pkl" | "python" | "starlark"  — Monaco language id
    readonly?    boolean — disables edits
    filename?    string  — informational only; not used by Monaco

  Emits:
    update:modelValue  — fires on every keystroke

  Lifecycle:
    - Registers the Pkl language + Monarch grammar once globally
      (idempotent — guarded by a module-scope flag).
    - Creates the editor on mount, disposes on unmount.
    - Watches `modelValue` for external changes; only writes back
      if the new value differs from the current editor value (avoids
      infinite update loops).
-->
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import * as monaco from "monaco-editor";
import { pklLanguageId, pklLanguageConfig, pklMonarchTokens } from "./pkl-grammar";
import { starlarkLanguageId, starlarkLanguageConfig, starlarkMonarchTokens } from "./starlark-grammar";
import { setupStarlarkProviders } from "./starlark-providers";

const props = defineProps<{
  modelValue: string;
  language: "pkl" | "python" | "starlark";
  readonly?: boolean;
  filename?: string;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
}>();

const hostEl = ref<HTMLDivElement | null>(null);
let editor: monaco.editor.IStandaloneCodeEditor | null = null;

let pklRegistered = false;
function ensurePklRegistered(): void {
  if (pklRegistered) return;
  pklRegistered = true;
  monaco.languages.register({ id: pklLanguageId });
  monaco.languages.setLanguageConfiguration(pklLanguageId, pklLanguageConfig);
  monaco.languages.setMonarchTokensProvider(pklLanguageId, pklMonarchTokens);
}

let starlarkRegistered = false;
function ensureStarlarkRegistered(): void {
  if (starlarkRegistered) return;
  starlarkRegistered = true;
  monaco.languages.register({
    id: starlarkLanguageId,
    extensions: [".star"],
    aliases: ["Starlark", "starlark"],
  });
  monaco.languages.setLanguageConfiguration(starlarkLanguageId, starlarkLanguageConfig);
  monaco.languages.setMonarchTokensProvider(starlarkLanguageId, starlarkMonarchTokens);
  setupStarlarkProviders();
}

onMounted(() => {
  if (!hostEl.value) return;
  if (props.language === "pkl") ensurePklRegistered();
  if (props.language === "starlark") ensureStarlarkRegistered();
  editor = monaco.editor.create(hostEl.value, {
    value: props.modelValue,
    language: props.language,
    readOnly: props.readonly ?? false,
    automaticLayout: true,
    minimap: { enabled: false },
    fontSize: 13,
    scrollBeyondLastLine: false,
    tabSize: 2,
    "semanticHighlighting.enabled": true,
  });
  editor.onDidChangeModelContent(() => {
    const v = editor?.getValue() ?? "";
    if (v !== props.modelValue) emit("update:modelValue", v);
  });
});

onBeforeUnmount(() => {
  editor?.dispose();
  editor = null;
});

watch(() => props.modelValue, (next) => {
  if (!editor) return;
  if (editor.getValue() !== next) {
    editor.setValue(next);
  }
});

watch(() => props.language, (lang) => {
  if (!editor) return;
  if (lang === "starlark") ensureStarlarkRegistered();
  if (lang === "pkl") ensurePklRegistered();
  const model = editor.getModel();
  if (model) monaco.editor.setModelLanguage(model, lang);
});

watch(() => props.readonly, (ro) => {
  editor?.updateOptions({ readOnly: ro ?? false });
});

function setPosition(lineNumber: number, column = 1): void {
  if (!editor) return;
  editor.setPosition({ lineNumber, column });
  editor.revealLineInCenter(lineNumber);
  editor.focus();
}

defineExpose({ setPosition });
</script>

<template>
  <div ref="hostEl" class="sy-code-editor" />
</template>

<style scoped>
.sy-code-editor {
  width: 100%;
  height: 100%;
  min-height: 200px;
}
</style>
