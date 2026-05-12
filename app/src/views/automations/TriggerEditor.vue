<!--
  TriggerEditor — a single trigger row. Dynamic sub-form by kind.

  v1 supports state_changed, time, event, webhook. The shape mirrors
  proto TriggerConfig's oneof.
-->
<script setup lang="ts">
import { computed } from "vue";
import { SyText, SyButton, SyInput, SyIcon } from "@/lib";

export type TriggerKind = "state_changed" | "time" | "event" | "webhook";

export interface TriggerValue {
  kind: TriggerKind;
  /** state_changed */ entity?: string;  from?: string; to?: string; holdSeconds?: number;
  /** time */         cron?: string;
  /** event */        eventKind?: string;
  /** webhook */      path?: string;
}

const props = defineProps<{ modelValue: TriggerValue }>();
const emit = defineEmits<{
  (e: "update:modelValue", v: TriggerValue): void;
  (e: "remove"): void;
}>();

const v = computed<TriggerValue>({
  get: () => props.modelValue,
  set: (next) => emit("update:modelValue", next),
});

function setKind(k: TriggerKind): void {
  v.value = { kind: k };
}

function update<K extends keyof TriggerValue>(key: K, val: TriggerValue[K]): void {
  v.value = { ...v.value, [key]: val };
}
</script>

<template>
  <div class="te">
    <div class="te__head">
      <select :value="v.kind" @change="setKind(($event.target as HTMLSelectElement).value as TriggerKind)">
        <option value="state_changed">State changed</option>
        <option value="time">Time</option>
        <option value="event">Event</option>
        <option value="webhook">Webhook</option>
      </select>
      <SyButton intent="ghost" size="sm" @click="emit('remove')">
        <SyIcon name="close" :size="12" />
      </SyButton>
    </div>

    <template v-if="v.kind === 'state_changed'">
      <SyInput
        :model-value="v.entity ?? ''"
        placeholder="entity id (e.g. light.kitchen)"
        @update:model-value="(s: string) => update('entity', s)"
      />
      <SyInput :model-value="v.from ?? ''" placeholder="from (optional)" @update:model-value="(s: string) => update('from', s)" />
      <SyInput :model-value="v.to ?? ''" placeholder="to (optional)" @update:model-value="(s: string) => update('to', s)" />
      <SyInput
        :model-value="String(v.holdSeconds ?? '')"
        placeholder="hold seconds (optional)"
        @update:model-value="(s: string) => update('holdSeconds', s === '' ? undefined : Number(s))"
      />
    </template>

    <template v-else-if="v.kind === 'time'">
      <SyInput
        :model-value="v.cron ?? ''"
        placeholder="cron (e.g. 0 9 * * MON)"
        @update:model-value="(s: string) => update('cron', s)"
      />
    </template>

    <template v-else-if="v.kind === 'event'">
      <SyInput
        :model-value="v.eventKind ?? ''"
        placeholder="event kind"
        @update:model-value="(s: string) => update('eventKind', s)"
      />
    </template>

    <template v-else-if="v.kind === 'webhook'">
      <SyInput
        :model-value="v.path ?? ''"
        placeholder="webhook path"
        @update:model-value="(s: string) => update('path', s)"
      />
    </template>
  </div>
</template>

<style scoped>
.te { display: flex; flex-direction: column; gap: var(--sy-space-2); padding: var(--sy-space-2); border: 1px solid var(--sy-color-line-soft); border-radius: var(--sy-radius-sm); }
.te__head { display: flex; align-items: center; justify-content: space-between; gap: var(--sy-space-2); }
.te__head select { padding: 4px var(--sy-space-2); border: 1px solid var(--sy-color-line); border-radius: var(--sy-radius-sm); background: var(--sy-color-surface-2); color: var(--sy-color-fg); }
</style>
