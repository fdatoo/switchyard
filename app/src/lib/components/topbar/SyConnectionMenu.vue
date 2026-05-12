<!--
  SyConnectionMenu — popover content shown when the user clicks the
  daemon-status dot in the topbar. Displays connection state, last
  successful reload timestamp, and a "Reload config" button. If the
  last reload returned an error, it surfaces inline.
-->
<script setup lang="ts">
import { computed } from "vue";
import { SyText, SyButton } from "@/lib";
import { configStore } from "@/stores/config-store";

const props = defineProps<{
  daemonStatus: "ok" | "reconnecting" | "down" | "checking";
}>();

const emit = defineEmits<{
  close: [];
}>();

const lastReloadLabel = computed<string>(() => {
  const at = configStore.lastReloadAt.value;
  if (!at) return "Not yet reloaded this session";
  return new Date(at).toLocaleTimeString(undefined, {
    hour: "2-digit", minute: "2-digit", second: "2-digit",
  });
});

const statusLabel = computed<string>(() => {
  switch (props.daemonStatus) {
    case "ok":           return "Connected";
    case "reconnecting": return "Reconnecting…";
    case "down":         return "Disconnected";
    case "checking":     return "Checking…";
  }
});

const canReload = computed<boolean>(() => props.daemonStatus === "ok");

async function onReload(): Promise<void> {
  await configStore.triggerReload();
  if (!configStore.lastReloadError.value) {
    emit("close");
  }
}
</script>

<template>
  <div class="sy-conn-menu">
    <div class="sy-conn-menu__row">
      <SyText variant="label" tone="subtle">Daemon</SyText>
      <SyText variant="body">{{ statusLabel }}</SyText>
    </div>
    <div class="sy-conn-menu__row">
      <SyText variant="label" tone="subtle">Last reload</SyText>
      <SyText variant="body">{{ lastReloadLabel }}</SyText>
    </div>

    <div class="sy-conn-menu__sep" />

    <SyButton intent="ghost" size="sm" :disabled="!canReload" @click="onReload">
      Reload config
    </SyButton>

    <SyText
      v-if="configStore.lastReloadError.value"
      variant="caption"
      tone="bad"
      class="sy-conn-menu__err"
    >
      {{ configStore.lastReloadError.value }}
    </SyText>
  </div>
</template>

<style scoped>
.sy-conn-menu {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-2);
  padding: var(--sy-space-3);
  min-width: 240px;
}
.sy-conn-menu__row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--sy-space-3);
}
.sy-conn-menu__sep {
  height: 1px;
  background: var(--sy-color-line-soft);
  margin: var(--sy-space-1) 0;
}
.sy-conn-menu__err {
  margin-top: var(--sy-space-2);
  white-space: pre-wrap;
}
</style>
