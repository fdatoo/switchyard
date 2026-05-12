<script setup lang="ts">
import { ref } from "vue";
import { SyText, SyAmbientRoomTile } from "@/lib";
import type { SceneDef } from "@/lib/components/ambient-room-tile/SyAmbientRoomTile.vue";

const lastEvent = ref("");

function onSelect(room: string): void {
  lastEvent.value = `${room}: open detail`;
}
function onScene(room: string, sceneId: string): void {
  lastEvent.value = `${room}: scene ${sceneId}`;
}

const livingScenes: SceneDef[] = [
  { id: "movie",    label: "Movie time" },
  { id: "evening",  label: "Evening"    },
  { id: "all-off",  label: "All off"    },
];
const bedroomScenes: SceneDef[] = [
  { id: "wind-down", label: "Wind down" },
  { id: "all-off",   label: "All off" },
];
</script>

<template>
  <div class="specimen">
    <SyText variant="caption" tone="subtle">
      Last: <SyText as="span" weight="medium">{{ lastEvent || "—" }}</SyText>
    </SyText>

    <div class="block">
      <SyText variant="label" tone="subtle">Display grid · canonical mix</SyText>
      <div class="display-grid">
        <SyAmbientRoomTile
          name="Living Room"
          width="wide"
          metric="72°F · 3 lights on · Now playing"
          :scenes="livingScenes"
          @select="onSelect('Living Room')"
          @scene="(id) => onScene('Living Room', id)"
        />
        <SyAmbientRoomTile
          name="Kitchen"
          metric="2 lights on"
          @select="onSelect('Kitchen')"
        />
        <SyAmbientRoomTile
          name="Bedroom"
          metric="65°F · asleep"
          :scenes="bedroomScenes"
          @select="onSelect('Bedroom')"
          @scene="(id) => onScene('Bedroom', id)"
        />
        <SyAmbientRoomTile
          name="Office"
          metric="Motion 2 min ago"
          @select="onSelect('Office')"
        />
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Notice — doorbell / visitor / delivery</SyText>
      <div class="display-grid">
        <SyAmbientRoomTile
          name="Front Door"
          metric="Someone at the door"
          urgency="notice"
          urgency-label="DOORBELL"
          :scenes="[{ id: 'intercom', label: 'Talk' }, { id: 'unlock', label: 'Unlock' }]"
          @select="onSelect('Front Door')"
          @scene="(id) => onScene('Front Door', id)"
        />
        <SyAmbientRoomTile
          name="Porch"
          metric="Package delivered 2 min ago"
          urgency="notice"
          urgency-label="DELIVERY"
          @select="onSelect('Porch')"
        />
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Alert — fire / leak / intrusion</SyText>
      <div class="display-grid">
        <SyAmbientRoomTile
          name="Kitchen"
          metric="Smoke detected"
          urgency="alert"
          urgency-label="SMOKE"
          width="wide"
          :scenes="[{ id: 'silence', label: 'Silence alarm' }, { id: 'call', label: 'Call 911' }]"
          @select="onSelect('Kitchen (alert)')"
          @scene="(id) => onScene('Kitchen', id)"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-3); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }

.display-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--sy-space-3);
}
</style>
