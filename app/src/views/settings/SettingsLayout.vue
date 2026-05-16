<!--
  SettingsLayout — wrapper for /settings/* with a sub-nav rail on the left
  and the active section rendered into <RouterView /> on the right.

  Nests inside AppLayout (which provides the global sidebar + topbar). To
  keep visual hierarchy clear we render the sub-nav as a slim, transparent
  column of SyNavItem rows; the breadcrumb in the topbar shows
  "Settings > Section" via the standard crumbs derivation.
-->
<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { SyText, SyNavItem } from "@/lib";
import type { IconName } from "@/lib/components/icon/SyIcon.vue";

interface SectionItem {
  id: string;
  path: string;
  icon: IconName;
  label: string;
  /** Renders a `Soon` badge until the section is built. */
  stub?: boolean;
}

interface SectionGroup {
  heading: string;
  items: SectionItem[];
}

/* Single source of truth for the sub-nav. New section = new entry here
   + a new route + a new section view. Icons are drawn from the existing
   SyIcon set (no new icons added for this UI). */
const GROUPS: SectionGroup[] = [
  {
    heading: "Personal",
    items: [
      { id: "appearance", path: "/settings/appearance", icon: "sparkle",  label: "Appearance" },
      { id: "account",    path: "/settings/account",    icon: "home",     label: "Account" },
    ],
  },
  {
    heading: "System",
    items: [
      { id: "drivers",     path: "/settings/drivers",     icon: "plugin",   label: "Drivers" },
      { id: "pkl",         path: "/settings/pkl",         icon: "settings", label: "Pkl config",   stub: true },
      { id: "widget-packs",path: "/settings/widget-packs",icon: "rooms",    label: "Widget packs", stub: true },
      { id: "displays",    path: "/settings/displays",    icon: "displays", label: "Displays",     stub: true },
    ],
  },
];

const route = useRoute();
const router = useRouter();

const activeId = computed<string>(() => {
  const segs = route.path.split("/").filter(Boolean);
  return segs[1] ?? "appearance";
});

const editorRoute = computed<boolean>(() => activeId.value === "pkl" || activeId.value === "starlark");

function go(path: string): void {
  router.push(path);
}
</script>

<template>
  <div class="settings" :class="{ 'settings--editor': editorRoute }">
    <aside class="settings__rail" aria-label="Settings sections">
      <div v-for="g in GROUPS" :key="g.heading" class="settings__group">
        <SyText
          variant="overline"
          tone="subtle"
          weight="medium"
          class="settings__heading"
        >
          {{ g.heading }}
        </SyText>
        <SyNavItem
          v-for="item in g.items"
          :key="item.id"
          as="button"
          :icon="item.icon"
          :label="item.label"
          :active="activeId === item.id"
          :badge="item.stub ? { count: 'Soon', intent: 'neutral' } : undefined"
          @click="go(item.path)"
        />
      </div>
    </aside>

    <main class="settings__content">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.settings {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  /* Token set tops out at --sy-space-6 (2rem); the rail/content gap and
     outer inset both want more air than that, so they're explicit here. */
  gap: 3rem;
  padding: 2rem 3rem;
  min-height: 100%;
}

.settings__rail {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-5);
  position: sticky;
  top: var(--sy-space-5);
  align-self: start;
}

.settings__group {
  display: flex;
  flex-direction: column;
  /* Tight inter-item spacing — these are nav rows, not standalone cards. */
  gap: 2px;
}

.settings__heading {
  /* Section labels sit slightly inside the nav rows' content edge so the
     label hangs under each row's icon-and-label inset. */
  padding: 0 10px var(--sy-space-1);
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.settings__content {
  min-width: 0;
  max-width: 720px;
}

.settings--editor {
  grid-template-columns: 190px minmax(0, 1fr);
  gap: 1rem;
  padding: 1rem;
}

.settings--editor .settings__content {
  max-width: none;
  min-height: 0;
}
</style>
