<script setup lang="ts">
import { ref } from "vue";
import { SyText, SySidebar } from "@/lib";
import type {
  SidebarNavItem,
  SidebarLink,
  SidebarUser,
} from "@/lib/components/sidebar/SySidebar.vue";

const lastEvent = ref("");

const primary: SidebarNavItem[] = [
  { id: "home",        icon: "home",        label: "Home",        path: "/",            shortcut: "⌘1" },
  { id: "rooms",       icon: "rooms",       label: "Rooms",       path: "/rooms",       shortcut: "⌘2" },
  { id: "activity",    icon: "activity",    label: "Activity",    path: "/activity",    shortcut: "⌘3", badge: { count: 12, intent: "accent" } },
  { id: "automations", icon: "automations", label: "Automations", path: "/automations", shortcut: "⌘4" },
  { id: "devices",     icon: "devices",     label: "Devices",     path: "/devices",     shortcut: "⌘5", badge: { count: 3, intent: "warn" } },
  { id: "settings",    icon: "settings",    label: "Settings",    path: "/settings",    shortcut: "⌘," },
];

const pages: SidebarLink[] = [
  { id: "energy", label: "Energy & climate", path: "/pages/energy-climate" },
  { id: "kids",   label: "Kids' room",       path: "/pages/kids-room" },
];

const displays: SidebarLink[] = [
  { id: "wall",    label: "Kitchen wall", path: "/displays/kitchen-wall" },
  { id: "fridge",  label: "Fridge",       path: "/displays/fridge" },
];

const user: SidebarUser = { name: "Fynn Datoo" };

const activePath = ref("/activity");

function onNavigate(path: string): void {
  activePath.value = path;
  lastEvent.value = `navigate → ${path}`;
}
function onUserMenu(id: string): void {
  lastEvent.value = `user menu → ${id}`;
}
</script>

<template>
  <div class="specimen">
    <SyText variant="caption" tone="subtle">
      Last: <SyText as="span" weight="medium">{{ lastEvent || "—" }}</SyText>
    </SyText>

    <div class="block">
      <SyText variant="label" tone="subtle">Full sidebar · with content</SyText>
      <div class="frame">
        <SySidebar
          :primary="primary"
          :pages="pages"
          :displays="displays"
          :user="user"
          :active-path="activePath"
          @navigate="onNavigate"
          @user-menu="onUserMenu"
        />
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Empty sections · no user</SyText>
      <div class="frame">
        <SySidebar
          :primary="primary"
          :active-path="activePath"
          @navigate="onNavigate"
          @sign-in="lastEvent = 'sign-in'"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-3); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }

/* Frame the sidebar so it doesn't try to fill 100vh inside the lab cell. */
.frame {
  height: 540px;
  display: flex;
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius);
  overflow: hidden;
  background: var(--sy-color-bg);
}
.frame :deep(.sy-sidebar) {
  height: 100%;
  max-height: 100%;
}
</style>
