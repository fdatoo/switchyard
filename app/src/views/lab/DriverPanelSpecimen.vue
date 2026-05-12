<script setup lang="ts">
import { ref } from "vue";
import { SyText, SyDriverPanel, SySurface } from "@/lib";
import type {
  DriverInfo,
  DriverEventDef,
} from "@/lib/components/driver-panel/SyDriverPanel.vue";

const lastAction = ref("");

const hueDriver: DriverInfo = {
  name: "hue_main",
  pack: "switchyard/hue",
  version: "2.4.1",
  state: "running",
  stateDetail: "Connected for 4h 12m",
  entityCount: 12,
  entityTypes: [
    { type: "light", count: 9 },
    { type: "switch", count: 2 },
    { type: "sensor", count: 1 },
  ],
};

const hueEvents: DriverEventDef[] = [
  { id: "1", icon: "bulb", intent: "good", title: "light.kitchen_pendant turned on", meta: "brightness 80%", timestamp: "12:42" },
  { id: "2", icon: "bulb", intent: "good", title: "light.living_room_lamp turned on", meta: "brightness 60%", timestamp: "12:42" },
  { id: "3", icon: "bulb", intent: "neutral", title: "light.hallway_sconce turned off", timestamp: "11:58" },
];

const matterDriver: DriverInfo = {
  name: "matter_bridge",
  pack: "switchyard/matter",
  version: "1.0.3",
  state: "reconnecting",
  stateDetail: "Attempt 3 of 5 · next retry in 8s",
  entityCount: 4,
  entityTypes: [
    { type: "light", count: 2 },
    { type: "switch", count: 2 },
  ],
};

const stoppedDriver: DriverInfo = {
  name: "shelly_legacy",
  pack: "switchyard/shelly",
  version: "0.9.1",
  state: "stopped",
  stateDetail: "Last error: auth_failed at 11:08",
  entityCount: 0,
};

function on(name: string, action: string): void {
  lastAction.value = `${name}: ${action}`;
}
</script>

<template>
  <div class="specimen">
    <SyText variant="caption" tone="subtle">
      Last: <SyText as="span" weight="medium">{{ lastAction || "—" }}</SyText>
    </SyText>

    <div class="block">
      <SyText variant="label" tone="subtle">Driver detail · running</SyText>
      <SySurface padding="lg">
        <SyDriverPanel
          :driver="hueDriver"
          :recent-events="hueEvents"
          @restart="on('hue_main', 'restart')"
          @configure="on('hue_main', 'configure')"
          @view-logs="on('hue_main', 'view-logs')"
        />
      </SySurface>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Driver detail · reconnecting</SyText>
      <SySurface padding="lg">
        <SyDriverPanel
          :driver="matterDriver"
          @restart="on('matter_bridge', 'restart')"
          @configure="on('matter_bridge', 'configure')"
          @view-logs="on('matter_bridge', 'view-logs')"
        />
      </SySurface>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Driver detail · stopped</SyText>
      <SySurface padding="lg">
        <SyDriverPanel
          :driver="stoppedDriver"
          @restart="on('shelly_legacy', 'restart')"
          @configure="on('shelly_legacy', 'configure')"
          @view-logs="on('shelly_legacy', 'view-logs')"
        />
      </SySurface>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-3); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
</style>
