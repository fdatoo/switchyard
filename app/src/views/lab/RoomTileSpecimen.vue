<script setup lang="ts">
import { SyText, SyRoomTile, SyBadge, SyButton } from "@/lib";
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Rooms grid · canonical</SyText>
      <div class="grid">
        <SyRoomTile name="Living Room" meta="8 entities · 2 people present" href="#" @click.prevent>
          <SyBadge intent="good" dot>3 lights on</SyBadge>
          <SyBadge>68°F</SyBadge>
          <SyBadge intent="info">Now playing</SyBadge>
          <template #scenes>
            <SyButton intent="ghost" size="sm">Movie time</SyButton>
            <SyButton intent="ghost" size="sm">All off</SyButton>
          </template>
        </SyRoomTile>

        <SyRoomTile name="Kitchen" meta="5 entities" href="#" @click.prevent>
          <SyBadge intent="good" dot>2 lights on</SyBadge>
          <SyBadge>72°F</SyBadge>
        </SyRoomTile>

        <SyRoomTile name="Bedroom" meta="4 entities · asleep" href="#" @click.prevent>
          <SyBadge intent="neutral" dot>All off</SyBadge>
          <SyBadge>65°F</SyBadge>
          <template #scenes>
            <SyButton intent="ghost" size="sm">Sunrise</SyButton>
          </template>
        </SyRoomTile>

        <SyRoomTile name="Office" meta="6 entities" href="#" @click.prevent>
          <SyBadge intent="warn" dot>Motion 2 min ago</SyBadge>
          <SyBadge>70°F</SyBadge>
        </SyRoomTile>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Minimal — no stats, no scenes</SyText>
      <div class="grid">
        <SyRoomTile name="Garage" meta="0 entities" href="#" @click.prevent />
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: var(--sy-space-3);
}
</style>
