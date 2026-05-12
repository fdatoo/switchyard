<!--
  SyCodeEditorPanel — the editor's outer shell. Owns the
  open-edit-commit lifecycle and the file tree.

  Props:
    kind: "pkl" | "starlark"

  Layout: file tree (left), Monaco editor (center), status bar (top
  of the editor pane). Bottom slot for kind-specific panels
  (e.g., SyTestPanel for Starlark).
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import {
  SyText, SyButton, SyEmptyState, SyIcon, SySurface,
  SyCodeEditor, SyFileTree,
} from "@/lib";
import type { FileEntry as TreeEntry } from "@/lib/components/file-tree/SyFileTree.vue";
import {
  listFiles, openForEdit, commitEdit, abandonEdit, sessionEvents,
} from "@/data/edit-session";

const props = defineProps<{
  kind: "pkl" | "starlark";
}>();

const treeEntries = ref<TreeEntry[]>([]);
const treeError = ref<string>("");
const treeLoading = ref<boolean>(true);

const selectedPath = ref<string>("");
const buffer = ref<string>("");
const lastLoaded = ref<string>("");
const sessionId = ref<string>("");
const lockToken = ref<string>("");
const fileHash = ref<string>("");
const banner = ref<string>("");
const saveBusy = ref<boolean>(false);
const saveError = ref<string>("");

const dirty = computed<boolean>(() => buffer.value !== lastLoaded.value);
const language = computed<"pkl" | "python">(() => props.kind === "pkl" ? "pkl" : "python");
const fileExt = computed<"pkl" | "star">(() => props.kind === "pkl" ? "pkl" : "star");

let sessionAbort: AbortController | null = null;

async function loadTree(): Promise<void> {
  treeLoading.value = true;
  treeError.value = "";
  try {
    const r = await listFiles();
    treeEntries.value = r.files
      .filter((f) => f.kind === fileExt.value)
      .map((f): TreeEntry => {
        const name = f.path.split("/").pop() ?? f.path;
        return { path: f.path, name, kind: f.kind };
      });
  } catch (err) {
    treeError.value = err instanceof Error ? err.message : String(err);
  } finally {
    treeLoading.value = false;
  }
}

async function abandonCurrent(): Promise<void> {
  if (!sessionId.value || !lockToken.value || !selectedPath.value) return;
  try {
    await abandonEdit({ filePath: selectedPath.value, lockToken: lockToken.value });
  } catch { /* best-effort */ }
  sessionAbort?.abort();
  sessionAbort = null;
  sessionId.value = "";
  lockToken.value = "";
}

async function openFile(path: string): Promise<void> {
  if (dirty.value && !confirm("Discard unsaved changes?")) return;
  await abandonCurrent();
  banner.value = "";
  saveError.value = "";
  try {
    const r = await openForEdit(path);
    selectedPath.value = path;
    buffer.value = r.ancestorPkl;
    lastLoaded.value = r.ancestorPkl;
    sessionId.value = r.sessionId;
    lockToken.value = r.lockToken;
    fileHash.value = r.fileHash;
    startSessionStream();
  } catch (err) {
    saveError.value = err instanceof Error ? err.message : String(err);
  }
}

function startSessionStream(): void {
  sessionAbort?.abort();
  sessionAbort = new AbortController();
  const args = { sessionId: sessionId.value, lockToken: lockToken.value };
  const ac = sessionAbort;
  (async () => {
    try {
      for await (const ev of sessionEvents(args, { signal: ac.signal })) {
        if (ev.kind === "external_edit_detected") {
          banner.value = "This file changed on disk. Reload to reconcile.";
        }
      }
    } catch { /* reconnects are out of scope for v1 */ }
  })();
}

async function save(): Promise<void> {
  if (!sessionId.value || !lockToken.value || !selectedPath.value) return;
  saveBusy.value = true;
  saveError.value = "";
  try {
    const r = await commitEdit({
      filePath: selectedPath.value,
      lockToken: lockToken.value,
      regeneratedPkl: buffer.value,
      expectedFileHash: fileHash.value,
    });
    if (r.conflict) {
      banner.value = `Conflict: ${r.conflict.reason}. Reload to reconcile.`;
      return;
    }
    fileHash.value = r.newFileHash;
    lastLoaded.value = buffer.value;
  } catch (err) {
    saveError.value = err instanceof Error ? err.message : String(err);
  } finally {
    saveBusy.value = false;
  }
}

async function reload(): Promise<void> {
  if (!selectedPath.value) return;
  banner.value = "";
  await openFile(selectedPath.value);
}

function discard(): void {
  buffer.value = lastLoaded.value;
}

onMounted(loadTree);
onBeforeUnmount(() => { void abandonCurrent(); });

watch(() => props.kind, () => {
  selectedPath.value = "";
  buffer.value = "";
  lastLoaded.value = "";
  void loadTree();
});
</script>

<template>
  <div class="sy-panel">
    <!-- Status bar -->
    <header class="sy-panel__bar">
      <SyText as="span" variant="caption" weight="medium">
        {{ selectedPath || "no file selected" }}
      </SyText>
      <SyText v-if="dirty" as="span" variant="caption" tone="warn">● unsaved</SyText>
      <div class="sy-panel__barRight">
        <SyButton
          v-if="selectedPath"
          intent="ghost"
          size="sm"
          :disabled="!dirty || saveBusy"
          @click="discard"
        >Discard</SyButton>
        <SyButton
          v-if="selectedPath"
          intent="primary"
          size="sm"
          :disabled="!dirty || saveBusy"
          @click="save"
        >{{ saveBusy ? "Saving…" : "Save" }}</SyButton>
      </div>
    </header>

    <div v-if="banner" class="sy-panel__banner">
      <SyText variant="caption" tone="warn">{{ banner }}</SyText>
      <SyButton intent="ghost" size="sm" @click="reload">Reload</SyButton>
    </div>

    <div class="sy-panel__body">
      <aside class="sy-panel__tree">
        <SyEmptyState
          v-if="treeLoading"
          loading
          title="Loading files…"
        />
        <SyText v-else-if="treeError" variant="caption" tone="bad">{{ treeError }}</SyText>
        <SyFileTree
          v-else
          :entries="treeEntries"
          :selected-path="selectedPath"
          @select="openFile"
        />
      </aside>

      <main class="sy-panel__editor">
        <SyCodeEditor
          v-if="selectedPath"
          v-model="buffer"
          :language="language"
        />
        <SyEmptyState
          v-else
          title="Select a file"
          description="Pick a file from the tree to start editing."
        >
          <template #icon><SyIcon :name="kind === 'pkl' ? 'plugin' : 'automations'" :size="28" /></template>
        </SyEmptyState>
      </main>
    </div>

    <SyText v-if="saveError" variant="caption" tone="bad" class="sy-panel__saveErr">
      {{ saveError }}
    </SyText>

    <footer v-if="$slots.bottom" class="sy-panel__bottom">
      <slot name="bottom" :selectedPath="selectedPath" />
    </footer>
  </div>
</template>

<style scoped>
.sy-panel {
  display: grid;
  grid-template-rows: auto auto 1fr auto auto;
  height: 100%;
  min-height: 600px;
  gap: 0;
}
.sy-panel__bar {
  display: flex; align-items: center; gap: var(--sy-space-3);
  padding: var(--sy-space-2) var(--sy-space-3);
  border-bottom: 1px solid var(--sy-color-line-soft);
}
.sy-panel__barRight { margin-left: auto; display: flex; gap: var(--sy-space-2); }
.sy-panel__banner {
  display: flex; align-items: center; gap: var(--sy-space-3);
  padding: var(--sy-space-2) var(--sy-space-3);
  background: color-mix(in srgb, var(--sy-color-warn) 10%, transparent);
  border-bottom: 1px solid var(--sy-color-line-soft);
}
.sy-panel__body {
  display: grid; grid-template-columns: 220px 1fr; min-height: 0;
}
.sy-panel__tree {
  border-right: 1px solid var(--sy-color-line-soft);
  overflow-y: auto;
}
.sy-panel__editor { overflow: hidden; }
.sy-panel__saveErr { padding: var(--sy-space-2) var(--sy-space-3); }
.sy-panel__bottom {
  border-top: 1px solid var(--sy-color-line-soft);
  max-height: 240px; overflow-y: auto;
}
</style>
