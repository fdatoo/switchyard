<!--
  SyRoomTile — friendly-style room grid tile.

  Card-shaped tile for the Rooms grid on the Rooms page and the Home page's
  Rooms widget. Wraps SySurface in interactive mode (cursor-tracking hover
  + press-toward-cursor animation) so the whole card behaves as one big
  click target.

  Structure:
    - Header: large gradient avatar tile + room name + optional secondary
      meta line ("3 entities", "2 people present", etc.).
    - Body slot: consumer-composed status (typically SyBadges for `3 lights
      on`, `68°F`, `motion 2 min ago`).
    - Footer slot: optional scene/action chips ("Movie time", "All off").

  This is the friendly-language render. Ambient renders rooms as the much
  larger SyAmbientRoomTile (next component); developer renders rooms as a
  SyDataTable row. Same data, three different surfaces — that's the
  three-language architecture working as designed.
-->
<script setup lang="ts">
import { computed } from "vue";
import SySurface from "@/lib/components/surface/SySurface.vue";
import SyText from "@/lib/components/text/SyText.vue";
import SyAvatar from "@/lib/components/avatar/SyAvatar.vue";

const props = defineProps<{
  name: string;
  /** Secondary meta line under the room name. */
  meta?: string;
  /** Click target for the whole tile. */
  href?: string;
}>();

const emit = defineEmits<{
  /** Fires on tile click. Parents can call e.preventDefault() and route
   *  via vue-router for SPA navigation; the underlying <a href> remains
   *  for accessibility (right-click → open in new tab still works). */
  (e: "click", event: MouseEvent): void;
}>();

const isInteractive = computed(() => Boolean(props.href));

function onClick(e: MouseEvent): void {
  emit("click", e);
}
</script>

<template>
  <SySurface
    :as="href ? 'a' : 'div'"
    :href="href"
    elevation="raised"
    padding="md"
    :interactive="isInteractive"
    class="sy-room"
    @click="onClick"
  >
    <header class="sy-room__head">
      <SyAvatar :name="name" size="lg" shape="square" />
      <div class="sy-room__title">
        <SyText variant="subtitle" weight="semibold">{{ name }}</SyText>
        <SyText v-if="meta" variant="caption" tone="subtle">{{ meta }}</SyText>
      </div>
    </header>

    <div v-if="$slots.default" class="sy-room__stats">
      <slot />
    </div>

    <footer v-if="$slots.scenes" class="sy-room__scenes">
      <slot name="scenes" />
    </footer>
  </SySurface>
</template>

<style scoped>
.sy-room {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-3);
  /* SySurface is bordered + rounded; tile inherits those. We override the
     `as="a"` text-decoration that SySurface doesn't set since most
     surfaces aren't anchors. */
  text-decoration: none;
  color: var(--sy-color-fg);
  min-height: 140px;
}

.sy-room__head {
  display: flex;
  align-items: center;
  gap: var(--sy-space-3);
  min-width: 0;
}

.sy-room__title {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.sy-room__stats {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sy-space-2);
  /* Top divider line — subtle. Keeps the header visually distinct from
     the stats area without an extra surface. */
  padding-top: var(--sy-space-3);
  border-top: 1px solid var(--sy-color-line-soft);
}

.sy-room__scenes {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sy-space-1);
  margin-top: auto;
  /* Push scenes to the bottom of the tile so equal-height tiles in a grid
     align their scene rows. */
}
</style>
