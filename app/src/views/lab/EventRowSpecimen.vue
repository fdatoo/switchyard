<script setup lang="ts">
import { SyText, SyEventRow, SySurface } from "@/lib";
import type { IconName } from "@/lib/components/icon/SyIcon.vue";

interface EventDemo {
  icon: IconName;
  intent: "neutral" | "accent" | "good" | "warn" | "bad" | "info" | "automation";
  title: string;
  meta?: string;
  timestamp: string;
  cause?: string;
}

const feed: EventDemo[] = [
  {
    icon: "bulb",
    intent: "good",
    title: "light.kitchen_pendant turned on",
    meta: "brightness 80%, warm white",
    timestamp: "12:42",
    cause: "← sunset_lights",
  },
  {
    icon: "automations",
    intent: "automation",
    title: "automation.sunset_lights triggered",
    meta: "sun · −15 min",
    timestamp: "12:42",
  },
  {
    icon: "thermometer",
    intent: "info",
    title: "sensor.living_room reached 68°F",
    timestamp: "12:36",
  },
  {
    icon: "plugin",
    intent: "warn",
    title: "driver.matter_bridge reconnecting",
    meta: "3rd attempt · backoff 8s",
    timestamp: "12:31",
  },
  {
    icon: "power",
    intent: "bad",
    title: "switch.garage_door failed to respond",
    meta: "timeout after 3.0s",
    timestamp: "12:28",
  },
  {
    icon: "sparkle",
    intent: "accent",
    title: "scene.movie_time activated",
    timestamp: "Yesterday",
    cause: "← living_room_speakers",
  },
];
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Activity feed · canonical pattern</SyText>
      <SySurface padding="none">
        <div class="feed">
          <SyEventRow
            v-for="(e, i) in feed"
            :key="i"
            :icon="e.icon"
            :intent="e.intent"
            :title="e.title"
            :meta="e.meta"
            :timestamp="e.timestamp"
            :cause="e.cause"
            interactive
          />
        </div>
      </SySurface>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Single row · with cause</SyText>
      <SySurface padding="none">
        <SyEventRow
          icon="bulb"
          intent="good"
          title="light.kitchen_pendant turned on"
          meta="brightness 80%"
          timestamp="2 min ago"
          cause="← sunset_lights"
        />
      </SySurface>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }

.feed {
  display: flex;
  flex-direction: column;
}
/* Thin separator between feed rows. */
.feed > :deep(.sy-event:not(:last-child)) {
  border-bottom: 1px solid var(--sy-color-line-soft);
}
</style>
