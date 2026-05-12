<script setup lang="ts">
import { ref } from "vue";
import { SyText, SyStoryRow, SyEventRow, SySurface } from "@/lib";
import type { IconName } from "@/lib/components/icon/SyIcon.vue";

interface StoryDemo {
  id: string;
  icon: IconName;
  intent: "neutral" | "accent" | "good" | "warn" | "bad" | "info" | "automation";
  title: string;
  meta: string;
  count: number;
  timestamp: string;
}

const stories: StoryDemo[] = [
  {
    id: "sunset",
    icon: "sparkle",
    intent: "automation",
    title: "Sunset routine ran",
    meta: "kitchen, living room, hallway",
    count: 4,
    timestamp: "12:42 — 12:47",
  },
  {
    id: "presence",
    icon: "home",
    intent: "good",
    title: "You arrived home",
    meta: "Welcome scene activated · 6 entity changes",
    count: 6,
    timestamp: "8:12 — 8:14",
  },
  {
    id: "reconnect",
    icon: "plugin",
    intent: "warn",
    title: "matter_bridge reconnected after 12 attempts",
    meta: "first failure 11:08 · resolved 11:14",
    count: 13,
    timestamp: "11:08 — 11:14",
  },
];

const expandedId = ref<string | null>("sunset");
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Activity · Stories tab</SyText>
      <SySurface padding="none">
        <div class="feed">
          <SyStoryRow
            v-for="s in stories"
            :key="s.id"
            :icon="s.icon"
            :intent="s.intent"
            :title="s.title"
            :meta="s.meta"
            :count="s.count"
            :timestamp="s.timestamp"
            :expanded="expandedId === s.id"
            interactive
            @toggle="expandedId = expandedId === s.id ? null : s.id"
          >
            <template v-if="s.id === 'sunset'" #events>
              <SyEventRow
                icon="automations"
                intent="automation"
                title="automation.sunset_lights triggered"
                meta="sun · −15 min"
                timestamp="12:42:01"
              />
              <SyEventRow
                icon="bulb"
                intent="good"
                title="light.kitchen_pendant turned on"
                meta="brightness 80%, warm white"
                timestamp="12:42:02"
                cause="← sunset_lights"
              />
              <SyEventRow
                icon="bulb"
                intent="good"
                title="light.living_room_lamp turned on"
                meta="brightness 60%"
                timestamp="12:42:02"
                cause="← sunset_lights"
              />
              <SyEventRow
                icon="bulb"
                intent="good"
                title="light.hallway_sconce turned on"
                meta="brightness 40%"
                timestamp="12:42:03"
                cause="← sunset_lights"
              />
            </template>
          </SyStoryRow>
        </div>
      </SySurface>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Static (no toggle)</SyText>
      <SySurface padding="none">
        <SyStoryRow
          icon="thermometer"
          intent="info"
          title="Living room temperature stabilized"
          meta="68°F ± 0.5° for 30 min"
          :count="12"
          timestamp="11:30 — 12:00"
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
.feed > :deep(.sy-story:not(:last-child)) {
  border-bottom: 1px solid var(--sy-color-line-soft);
}
</style>
