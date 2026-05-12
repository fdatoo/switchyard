<!--
  AutomationsView — list of declared automations with quick controls.

  Each row is a SyAutomationCard with:
    - Enable/disable toggle wired to AutomationService/Enable+Disable
    - Overflow menu: Run now (Trigger), Edit, Duplicate, Delete

  Today only Enable/Disable/Trigger are wired — Edit/Duplicate/Delete
  TODO when those RPCs exist or when the Pkl-editor sub-shell lands.

  Trigger uses optimistic UI: refresh after the RPC settles. The
  AutomationService doesn't expose an "is running" state directly; we
  trust the in_flight field on the next List.
-->
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import {
  SyText, SySurface, SyButton, SyIcon, SyEmptyState, SyAutomationCard,
} from "@/lib";
import {
  listAutomations, enableAutomation, disableAutomation, triggerAutomation,
  type Automation,
} from "@/data/automations";
import SyAutomationForm from "@/views/automations/SyAutomationForm.vue";

type LoadState = "loading" | "ok" | "error";

const automations = ref<Automation[]>([]);
const state = ref<LoadState>("loading");
const errorMessage = ref<string>("");
const actionError = ref<string>("");

let abort: AbortController | null = null;

async function load(): Promise<void> {
  abort?.abort();
  abort = new AbortController();
  state.value = "loading";
  errorMessage.value = "";
  try {
    const r = await listAutomations({ signal: abort.signal });
    automations.value = r.automations;
    state.value = "ok";
  } catch (err) {
    if ((err as Error).name === "AbortError") return;
    state.value = "error";
    errorMessage.value = err instanceof Error ? err.message : String(err);
  }
}

/** Silent re-fetch — preserves the page so toggling doesn't flash. */
async function refresh(): Promise<void> {
  try {
    const r = await listAutomations();
    automations.value = r.automations;
  } catch { /* next refresh will retry */ }
}

onMounted(load);
onBeforeUnmount(() => abort?.abort());

async function onToggle(a: Automation, next: boolean): Promise<void> {
  /* Optimistic local flip; revert on failure. */
  const idx = automations.value.findIndex((x) => x.id === a.id);
  if (idx === -1) return;
  const prev = automations.value[idx];
  automations.value[idx] = { ...prev, enabled: next };
  actionError.value = "";
  try {
    if (next) await enableAutomation(a.id);
    else      await disableAutomation(a.id);
    await refresh();
  } catch (err) {
    automations.value[idx] = prev;
    actionError.value = err instanceof Error ? err.message : String(err);
  }
}

async function onMenu(a: Automation, id: string): Promise<void> {
  actionError.value = "";
  try {
    switch (id) {
      case "run":
        await triggerAutomation(a.id);
        await refresh();
        break;
      case "duplicate":
      case "delete":
        /* TODO when the corresponding RPCs ship. */
        actionError.value = `${id} isn't wired yet`;
        break;
    }
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : String(err);
  }
}

/* ── Form (+ New / Edit) ─────────────────────────────────────────── */

const formOpen = ref<boolean>(false);
const formInitial = ref<undefined>(undefined);

function openNew(): void {
  formInitial.value = undefined;
  formOpen.value = true;
}

function openEdit(_id: string): void {
  // v1: prefill is deferred. Open the form blank; user re-enters id
  // matching the existing automation. The id-based filename means
  // saving will overwrite the existing automation correctly.
  formInitial.value = undefined;
  formOpen.value = true;
}

async function onSaved(_id: string): Promise<void> {
  await refresh();
}
</script>

<template>
  <div class="page">
    <header class="page__head">
      <div class="page__headLeft">
        <SyText as="h1" variant="display">Automations</SyText>
        <SyText v-if="state === 'ok'" variant="body" tone="subtle">
          {{ automations.length }}
          {{ automations.length === 1 ? "automation" : "automations" }}
        </SyText>
      </div>
      <div class="page__headRight">
        <SyButton intent="primary" @click="openNew">+ New</SyButton>
      </div>
    </header>

    <SySurface v-if="state === 'loading'" padding="none">
      <SyEmptyState loading title="Loading automations…" />
    </SySurface>

    <SySurface v-else-if="state === 'error'" padding="none">
      <SyEmptyState
        intent="bad"
        title="Couldn't load automations"
        :description="errorMessage"
      >
        <template #icon><SyIcon name="close" :size="28" /></template>
        <template #actions>
          <SyButton intent="secondary" @click="load">Retry</SyButton>
        </template>
      </SyEmptyState>
    </SySurface>

    <SySurface v-else-if="automations.length === 0" padding="none">
      <SyEmptyState
        title="No automations yet"
        description="Automations are written in Starlark and declared in Pkl config. Add one to schedule routines or react to events."
      >
        <template #icon><SyIcon name="automations" :size="28" /></template>
        <template #actions>
          <SyButton intent="primary">Open Pkl config</SyButton>
        </template>
      </SyEmptyState>
    </SySurface>

    <SySurface v-else padding="none" class="page__list">
      <SyAutomationCard
        v-for="a in automations"
        :key="a.id"
        :name="a.displayName"
        :trigger="a.mode || 'manual'"
        :enabled="a.enabled"
        :running="a.inFlight > 0"
        @toggle-enabled="(v) => onToggle(a, v)"
        @menu-action="(id) => id === 'edit' ? openEdit(a.id) : onMenu(a, id)"
      />
    </SySurface>

    <SyText
      v-if="actionError"
      variant="caption"
      tone="bad"
      class="page__actionError"
    >
      {{ actionError }}
    </SyText>

    <SyAutomationForm v-model:open="formOpen" :initial="formInitial" @saved="onSaved" />
  </div>
</template>

<style scoped>
.page {
  padding: var(--sy-space-5) var(--sy-space-6);
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-4);
  max-width: 1080px;
}
.page__head {
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}
.page__headLeft {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-1);
}
.page__list :deep(.sy-listrow + .sy-listrow) {
  border-top: 1px solid var(--sy-color-line-soft);
}
.page__actionError {
  margin-top: var(--sy-space-2);
}
</style>
