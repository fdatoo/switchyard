<!--
  SyTestPanel — streaming Starlark test runner. The host component
  starts the run by calling `start(scriptId)`. Stream events
  populate a flat list of rows; cancellation aborts the stream.
-->
<script setup lang="ts">
import { ref } from "vue";
import { SyText, SyButton, SyIcon } from "@/lib";
import { runTests, type TestEvent } from "@/data/script-service";

defineProps<{
  scriptId: string;
}>();

interface Row {
  name: string;
  state: "running" | "pass" | "fail";
  durationMs?: number;
  message?: string;
}

const rows = ref<Row[]>([]);
const running = ref<boolean>(false);
const summary = ref<{ passed: number; failed: number } | null>(null);
let abort: AbortController | null = null;

async function start(scriptId: string): Promise<void> {
  if (running.value) return;
  abort?.abort();
  abort = new AbortController();
  rows.value = [];
  summary.value = null;
  running.value = true;
  try {
    for await (const ev of runTests(scriptId, { signal: abort.signal })) {
      applyEvent(ev);
    }
  } catch (err) {
    if ((err as Error).name !== "AbortError") {
      rows.value.push({ name: "error", state: "fail", message: String(err) });
    }
  } finally {
    running.value = false;
  }
}

function applyEvent(ev: TestEvent): void {
  if (ev.kind === "start") {
    rows.value = [...rows.value, { name: ev.name, state: "running" }];
    return;
  }
  if (ev.kind === "pass" || ev.kind === "fail") {
    const idx = rows.value.findIndex((r) => r.name === ev.name && r.state === "running");
    const next = [...rows.value];
    if (idx >= 0) {
      next[idx] = ev.kind === "pass"
        ? { name: ev.name, state: "pass", durationMs: ev.durationMs }
        : { name: ev.name, state: "fail", message: ev.message };
    } else {
      next.push(ev.kind === "pass"
        ? { name: ev.name, state: "pass", durationMs: ev.durationMs }
        : { name: ev.name, state: "fail", message: ev.message });
    }
    rows.value = next;
    return;
  }
  // done
  summary.value = { passed: ev.passed, failed: ev.failed };
}

function cancel(): void {
  abort?.abort();
}

defineExpose({ start, cancel });
</script>

<template>
  <div class="sy-tests">
    <div class="sy-tests__head">
      <SyText variant="label" tone="subtle">Tests</SyText>
      <SyButton
        v-if="!running"
        intent="primary"
        size="sm"
        :disabled="!scriptId"
        @click="start(scriptId)"
      >
        Run tests
      </SyButton>
      <SyButton v-else intent="ghost" size="sm" @click="cancel">Cancel</SyButton>
    </div>

    <div v-if="rows.length === 0 && !running" class="sy-tests__empty">
      <SyText variant="caption" tone="subtle">No runs yet.</SyText>
    </div>

    <ul class="sy-tests__rows">
      <li v-for="r in rows" :key="r.name" :class="['sy-tests__row', `sy-tests__row--${r.state}`]">
        <SyIcon
          :name="r.state === 'pass' ? 'check' : r.state === 'fail' ? 'close' : 'activity'"
          :size="12"
        />
        <SyText as="span" variant="caption" weight="medium">{{ r.name }}</SyText>
        <SyText v-if="r.durationMs != null" as="span" variant="caption" tone="subtle">
          {{ r.durationMs }}ms
        </SyText>
        <SyText v-if="r.message" as="span" variant="caption" tone="bad">
          {{ r.message }}
        </SyText>
      </li>
    </ul>

    <SyText v-if="summary" variant="caption" tone="subtle">
      {{ summary.passed }} passed, {{ summary.failed }} failed
    </SyText>
  </div>
</template>

<style scoped>
.sy-tests { display: flex; flex-direction: column; gap: var(--sy-space-2); padding: var(--sy-space-3); }
.sy-tests__head { display: flex; align-items: center; justify-content: space-between; }
.sy-tests__empty { padding: var(--sy-space-2) 0; }
.sy-tests__rows { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }
.sy-tests__row { display: flex; align-items: center; gap: var(--sy-space-2); padding: 2px var(--sy-space-2); border-radius: var(--sy-radius-sm); }
.sy-tests__row--pass :first-child { color: var(--sy-color-good); }
.sy-tests__row--fail :first-child { color: var(--sy-color-bad); }
</style>
