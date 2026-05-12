<script setup lang="ts">
import { reactive, ref } from "vue";
import { SyText, SyAutomationCard, SySurface } from "@/lib";

interface AutomationDemo {
  id: string;
  name: string;
  trigger: string;
  enabled: boolean;
  running: boolean;
  nextRun?: string;
  lastRun?: string;
}

const automations = reactive<AutomationDemo[]>([
  {
    id: "sunset",
    name: "Sunset lights",
    trigger: "Sun · −15 min",
    enabled: true,
    running: false,
    nextRun: "In 4 h",
  },
  {
    id: "morning",
    name: "Morning routine",
    trigger: "Daily at 6:30 AM",
    enabled: true,
    running: true,
  },
  {
    id: "motion",
    name: "Hallway motion light",
    trigger: "When sensor.hallway_motion detected",
    enabled: true,
    running: false,
    nextRun: "Idle",
  },
  {
    id: "away",
    name: "Away mode",
    trigger: "When everyone leaves",
    enabled: false,
    running: false,
    lastRun: "Yesterday at 8:12",
  },
]);

const lastAction = ref<string>("");

function onToggle(id: string, next: boolean): void {
  const a = automations.find((x) => x.id === id);
  if (a) a.enabled = next;
  lastAction.value = `${id}: toggle → ${next}`;
}

function onMenuAction(id: string, action: string): void {
  lastAction.value = `${id}: ${action}`;
  if (action === "run") {
    const a = automations.find((x) => x.id === id);
    if (a) {
      a.running = true;
      setTimeout(() => { a.running = false; }, 1800);
    }
  }
}
</script>

<template>
  <div class="specimen">
    <SyText variant="caption" tone="subtle">
      Last action: <SyText as="span" weight="medium">{{ lastAction || "—" }}</SyText>
    </SyText>

    <div class="block">
      <SyText variant="label" tone="subtle">Automations index · canonical</SyText>
      <SySurface padding="none">
        <div class="list">
          <SyAutomationCard
            v-for="a in automations"
            :key="a.id"
            :name="a.name"
            :trigger="a.trigger"
            :enabled="a.enabled"
            :running="a.running"
            :next-run="a.nextRun"
            :last-run="a.lastRun"
            href="#open-detail"
            @click.prevent
            @toggle-enabled="onToggle(a.id, $event)"
            @menu-action="onMenuAction(a.id, $event)"
          />
        </div>
      </SySurface>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-3); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.list { display: flex; flex-direction: column; }
.list > :deep(.sy-auto:not(:last-child)) {
  border-bottom: 1px solid var(--sy-color-line-soft);
}
</style>
