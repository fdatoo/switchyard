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
import { useLanguageStore } from "@/lib/theme/language-store";
import { pklLanguageId, pklLanguageConfig, pklMonarchTokens } from "./pkl-grammar";
import { setupPklProviders } from "./pkl-providers";
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
const themeStore = useLanguageStore();

let pklRegistered = false;
function ensurePklRegistered(): void {
  if (pklRegistered) return;
  pklRegistered = true;
  monaco.languages.register({ id: pklLanguageId });
  monaco.languages.setLanguageConfiguration(pklLanguageId, pklLanguageConfig);
  monaco.languages.setMonarchTokensProvider(pklLanguageId, pklMonarchTokens);
  setupPklProviders();
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
  applyMonacoTheme();
  editor = monaco.editor.create(hostEl.value, {
    value: props.modelValue,
    language: props.language,
    readOnly: props.readonly ?? false,
    theme: currentMonacoThemeName(),
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

watch([() => themeStore.language, () => themeStore.mode], () => {
  window.requestAnimationFrame(applyMonacoTheme);
});

function setPosition(lineNumber: number, column = 1): void {
  if (!editor) return;
  editor.setPosition({ lineNumber, column });
  editor.revealLineInCenter(lineNumber);
  editor.focus();
}

defineExpose({ setPosition });

function currentMonacoThemeName(): string {
  return themeStore.mode === "dark" ? "sy-monaco-dark" : "sy-monaco-light";
}

function applyMonacoTheme(): void {
  const dark = themeStore.mode === "dark";
  const root = getComputedStyle(document.documentElement);
  const bg = cssColor(root, "--sy-color-surface-1", dark ? "#1e1e1e" : "#ffffff");
  const fg = cssColor(root, "--sy-color-fg", dark ? "#d4d4d4" : "#1a1a1f");
  const fg2 = cssColor(root, "--sy-color-fg-2", dark ? "#c8c8c8" : "#3a3a40");
  const fg3 = cssColor(root, "--sy-color-fg-3", dark ? "#9a9a9a" : "#6a6557");
  const line = cssColor(root, "--sy-color-line-soft", dark ? "#2a2a2a" : "#efeadf");
  const accent = cssColor(root, "--sy-color-accent", dark ? "#7adcff" : "#d97757");

  monaco.editor.defineTheme(currentMonacoThemeName(), {
    base: dark ? "vs-dark" : "vs",
    inherit: true,
    colors: {
      "editor.background": bg,
      "editor.foreground": fg,
      "editorLineNumber.foreground": fg3,
      "editorLineNumber.activeForeground": fg2,
      "editorCursor.foreground": accent,
      "editor.selectionBackground": withAlpha(accent, dark ? "55" : "33"),
      "editor.inactiveSelectionBackground": withAlpha(accent, dark ? "33" : "1f"),
      "editor.lineHighlightBackground": withAlpha(line, dark ? "66" : "80"),
      "editorGutter.background": bg,
    },
    rules: [
      { token: "comment", foreground: stripHash(fg3), fontStyle: "italic" },
      { token: "keyword", foreground: stripHash(accent), fontStyle: "bold" },
      { token: "string", foreground: stripHash(cssColor(root, "--sy-color-good", dark ? "#6ed09a" : "#4ca87a")) },
      { token: "number", foreground: stripHash(cssColor(root, "--sy-color-info", dark ? "#7ab4d8" : "#4c87b5")) },
      { token: "type", foreground: stripHash(cssColor(root, "--sy-color-purple", dark ? "#b48cff" : "#9b7ec8")) },
      { token: "class", foreground: stripHash(cssColor(root, "--sy-color-purple", dark ? "#b48cff" : "#9b7ec8")) },
      { token: "function", foreground: stripHash(cssColor(root, "--sy-color-warn", dark ? "#ebb168" : "#d99340")) },
      { token: "method", foreground: stripHash(cssColor(root, "--sy-color-warn", dark ? "#ebb168" : "#d99340")) },
      { token: "property", foreground: stripHash(fg2) },
      { token: "variable", foreground: stripHash(fg) },
    ],
  });
  monaco.editor.setTheme(currentMonacoThemeName());
}

function cssColor(styles: CSSStyleDeclaration, name: string, fallback: string): string {
  const value = styles.getPropertyValue(name).trim();
  return value || fallback;
}

function stripHash(color: string): string {
  return color.startsWith("#") ? color.slice(1) : color;
}

function withAlpha(color: string, alphaHex: string): string {
  if (/^#[0-9a-fA-F]{6}$/.test(color)) return `${color}${alphaHex}`;
  return color;
}
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
