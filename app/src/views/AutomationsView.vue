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
      case "edit":
      case "duplicate":
      case "delete":
        /* TODO when the corresponding RPCs / Pkl-editor flow ship. */
        actionError.value = `${id} isn't wired yet`;
        break;
    }
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : String(err);
  }
}
</script>

<template>
  <div class="page">
    <header class="page__head">
      <SyText as="h1" variant="display">Automations</SyText>
      <SyText v-if="state === 'ok'" variant="body" tone="subtle">
        {{ automations.length }}
        {{ automations.length === 1 ? "automation" : "automations" }}
      </SyText>
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
        @menu-action="(id) => onMenu(a, id)"
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
