<!--
  SyTestPanel — streaming Starlark test runner.

  Daemon protocol: one StarlarkTestEvent per test (outcome = "ok" or
  "fail"), no start/done sentinels. Stream-close marks run-finished.
  Prop `path` is the script file path the daemon should run tests on.
-->
<script setup lang="ts">
import { computed, ref } from "vue";
import { SyText, SyButton, SyIcon } from "@/lib";
import { runTests, type TestEvent } from "@/data/script-service";

defineProps<{
  path: string;
}>();

interface Row {
  name: string;
  state: "pass" | "fail";
  detail: string;
}

const rows = ref<Row[]>([]);
const running = ref<boolean>(false);
const error = ref<string>("");
let abort: AbortController | null = null;

const passed = computed<number>(() => rows.value.filter((r) => r.state === "pass").length);
const failed = computed<number>(() => rows.value.filter((r) => r.state === "fail").length);

async function start(path: string): Promise<void> {
  if (running.value || !path) return;
  abort?.abort();
  abort = new AbortController();
  rows.value = [];
  error.value = "";
  running.value = true;
  try {
    for await (const ev of runTests(path, { signal: abort.signal })) {
      applyEvent(ev);
    }
  } catch (err) {
    if ((err as Error).name !== "AbortError") {
      error.value = String(err);
    }
  } finally {
    running.value = false;
  }
}

function applyEvent(ev: TestEvent): void {
  rows.value = [...rows.value, { name: ev.name, state: ev.kind, detail: ev.detail }];
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
        :disabled="!path"
        @click="start(path)"
      >
        Run tests
      </SyButton>
      <SyButton v-else intent="ghost" size="sm" @click="cancel">Cancel</SyButton>
    </div>

    <div v-if="rows.length === 0 && !running && !error" class="sy-tests__empty">
      <SyText variant="caption" tone="subtle">No runs yet.</SyText>
    </div>

    <ul v-if="rows.length > 0" class="sy-tests__rows">
      <li v-for="(r, i) in rows" :key="`${r.name}-${i}`" :class="['sy-tests__row', `sy-tests__row--${r.state}`]">
        <SyIcon :name="r.state === 'pass' ? 'check' : 'close'" :size="12" />
        <SyText as="span" variant="caption" weight="medium">{{ r.name }}</SyText>
        <SyText v-if="r.detail" as="span" variant="caption" tone="subtle">
          {{ r.detail }}
        </SyText>
      </li>
    </ul>

    <SyText v-if="!running && rows.length > 0" variant="caption" tone="subtle">
      {{ passed }} passed, {{ failed }} failed
    </SyText>

    <SyText v-if="error" variant="caption" tone="bad">{{ error }}</SyText>
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
