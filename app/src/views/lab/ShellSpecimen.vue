<script setup lang="ts">
import { ref, computed } from "vue";
import { SyText, SyShell, SyAutomationCard, SyEventRow, SySurface, SyButton } from "@/lib";
import type { SidebarNavItem } from "@/lib/components/sidebar/SySidebar.vue";
import type { BreadcrumbItem } from "@/lib/components/breadcrumb/SyBreadcrumb.vue";

const lastEvent = ref("");

const primary: SidebarNavItem[] = [
  { id: "home",        icon: "home",        label: "Home",        path: "/",            shortcut: "⌘1" },
  { id: "rooms",       icon: "rooms",       label: "Rooms",       path: "/rooms",       shortcut: "⌘2" },
  { id: "activity",    icon: "activity",    label: "Activity",    path: "/activity",    shortcut: "⌘3", badge: { count: 12, intent: "accent" } },
  { id: "automations", icon: "automations", label: "Automations", path: "/automations", shortcut: "⌘4" },
  { id: "devices",     icon: "devices",     label: "Devices",     path: "/devices",     shortcut: "⌘5", badge: { count: 3, intent: "warn" } },
  { id: "settings",    icon: "settings",    label: "Settings",    path: "/settings",    shortcut: "⌘," },
];

const activePath = ref("/activity");

const crumbs = computed<BreadcrumbItem[]>(() => {
  switch (activePath.value) {
    case "/":            return [{ label: "Home" }];
    case "/rooms":       return [{ label: "Rooms" }];
    case "/activity":    return [{ label: "Activity" }];
    case "/automations": return [{ label: "Automations" }];
    case "/devices":     return [{ label: "Devices" }];
    case "/settings":    return [{ label: "Settings" }];
    default:             return [{ label: "Home" }];
  }
});

const daemonStatus = ref<"ok" | "reconnecting" | "down" | "checking">("ok");
</script>

<template>
  <div class="specimen">
    <SyText variant="caption" tone="subtle">
      Last: <SyText as="span" weight="medium">{{ lastEvent || "—" }}</SyText>
    </SyText>

    <div class="controls">
      <SyText variant="label" tone="subtle">Daemon:</SyText>
      <SyButton size="sm" :intent="daemonStatus === 'ok' ? 'primary' : 'ghost'" @click="daemonStatus = 'ok'">ok</SyButton>
      <SyButton size="sm" :intent="daemonStatus === 'reconnecting' ? 'primary' : 'ghost'" @click="daemonStatus = 'reconnecting'">reconnecting</SyButton>
      <SyButton size="sm" :intent="daemonStatus === 'down' ? 'primary' : 'ghost'" @click="daemonStatus = 'down'">down</SyButton>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Full shell · framed</SyText>
      <div class="frame">
        <SyShell
          :primary="primary"
          :user="{ name: 'Fynn Datoo' }"
          :active-path="activePath"
          :crumbs="crumbs"
          :daemon-status="daemonStatus"
          @navigate="(p) => { activePath = p; lastEvent = `navigate → ${p}`; }"
          @search="lastEvent = 'search'"
          @user-menu="(id) => lastEvent = `user-menu → ${id}`"
        >
          <!-- Page content stub — the active route's view goes here. -->
          <div class="page">
            <SyText as="h1" variant="display">{{ crumbs[crumbs.length - 1].label }}</SyText>
            <SyText variant="body" tone="subtle">
              In a real app, this is where the route's view component renders. The shell
              handles the chrome — sidebar, topbar, banner, scroll — and your view fills
              the content area.
            </SyText>
            <SySurface padding="md">
              <SyAutomationCard
                name="Sunset lights"
                trigger="Sun · −15 min"
                next-run="In 4 h"
                @click.prevent
              />
            </SySurface>
            <SySurface padding="none">
              <SyEventRow
                icon="bulb"
                intent="good"
                title="light.kitchen_pendant turned on"
                meta="brightness 80%"
                timestamp="12:42"
              />
              <SyEventRow
                icon="automations"
                intent="automation"
                title="automation.sunset_lights triggered"
                meta="sun · −15 min"
                timestamp="12:42"
              />
            </SySurface>
          </div>
        </SyShell>
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-3); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }

.controls {
  display: flex;
  align-items: center;
  gap: var(--sy-space-2);
}

/* Frame the shell so it doesn't try to fill 100vh inside the lab cell.
   `:deep(.sy-shell) { height: 100% }` overrides the shell's own 100vh. */
.frame {
  height: 600px;
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius);
  overflow: hidden;
}
.frame :deep(.sy-shell) { height: 100%; }

.page {
  padding: var(--sy-space-5);
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-4);
}
</style>
