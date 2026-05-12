<!--
  AppLayout — wraps child routes in SyShell.

  Owns the global command palette: pre-loads its catalog (drivers /
  automations / areas / nav) and listens for ⌘K (or ⌃K on
  non-mac) to open it. The shell's `search` event also opens the
  palette so the topbar search affordance is wired.

  Crumb derivation, polling, and the nav table all live in dedicated
  modules — this file is mostly glue.
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { SyShell, SyCommandPalette } from "@/lib";
import { useDaemonStatus } from "@/composables/use-daemon-status";
import { crumbsFor } from "@/router/crumbs";
import type { SidebarNavItem } from "@/lib/components/sidebar/SySidebar.vue";
import type { CommandItem } from "@/lib/components/command-palette/SyCommandPalette.vue";
import { listDrivers, type DriverSummary } from "@/data/driver-management";
import { listAutomations, type Automation } from "@/data/automations";
import { listAreas, type Area } from "@/data/areas";

const PRIMARY: SidebarNavItem[] = [
  { id: "home",        icon: "home",        label: "Home",        path: "/",            shortcut: "⌘1" },
  { id: "rooms",       icon: "rooms",       label: "Rooms",       path: "/rooms",       shortcut: "⌘2" },
  { id: "activity",    icon: "activity",    label: "Activity",    path: "/activity",    shortcut: "⌘3" },
  { id: "automations", icon: "automations", label: "Automations", path: "/automations", shortcut: "⌘4" },
  { id: "devices",     icon: "devices",     label: "Devices",     path: "/devices",     shortcut: "⌘5" },
  { id: "settings",    icon: "settings",    label: "Settings",    path: "/settings",    shortcut: "⌘," },
];

const route = useRoute();
const router = useRouter();
const { status: daemonStatus } = useDaemonStatus();

const crumbs = computed(() => crumbsFor(route.path, PRIMARY));

function onNavigate(path: string): void {
  router.push(path);
}

function onUserMenu(_id: string): void {
  /* TODO: wire account actions to AuthService once login is in scope. */
}

/* ---- Command palette ------------------------------------------------ */

const paletteOpen = ref<boolean>(false);
const drivers = ref<DriverSummary[]>([]);
const automations = ref<Automation[]>([]);
const areas = ref<Area[]>([]);

/* Pre-load the catalog so opening the palette is instant. Each fetch
   is independent and tolerant of failure — palette still renders the
   navigation entries (the most important category) even if everything
   else 500s. */
async function loadCatalog(): Promise<void> {
  void listDrivers().then((r) => { drivers.value = r.running; }).catch(() => {});
  void listAutomations().then((r) => { automations.value = r.automations; }).catch(() => {});
  void listAreas().then((r) => { areas.value = r.areas; }).catch(() => {});
}

/* Refresh when the palette opens so a long-running session still sees
   current data. The opening latency is hidden by the typing affordance
   — the list updates as items arrive. */
watch(paletteOpen, (open) => {
  if (open) loadCatalog();
});

/** Top-level catalog of palette entries. Order = render order. */
const items = computed<CommandItem[]>(() => {
  const out: CommandItem[] = [];

  /* Navigation: derive from PRIMARY so it stays in sync with the sidebar. */
  for (const nav of PRIMARY) {
    out.push({
      id: `nav:${nav.id}`,
      label: `Go to ${nav.label}`,
      group: "Navigation",
      icon: nav.icon,
      shortcut: nav.shortcut,
      description: nav.path,
    });
  }
  /* Settings sub-sections — keep parity with the SettingsLayout's
     left rail. */
  const settingsSubs: { id: string; label: string }[] = [
    { id: "appearance",   label: "Appearance" },
    { id: "account",      label: "Account" },
    { id: "drivers",      label: "Drivers" },
    { id: "pkl",          label: "Pkl config" },
    { id: "widget-packs", label: "Widget packs" },
    { id: "displays",     label: "Displays" },
  ];
  for (const s of settingsSubs) {
    out.push({
      id: `nav:settings/${s.id}`,
      label: `Settings · ${s.label}`,
      group: "Navigation",
      icon: "settings",
      description: `/settings/${s.id}`,
    });
  }

  /* Drivers — jump to Devices with the driver detail rail open. */
  for (const d of drivers.value) {
    out.push({
      id: `driver:${d.id}`,
      label: d.id,
      description: `Driver · ${d.state}`,
      group: "Drivers",
      icon: "plugin",
    });
  }

  /* Automations — jump to Automations page (per-automation detail TBD). */
  for (const a of automations.value) {
    out.push({
      id: `automation:${a.id}`,
      label: a.displayName,
      description: `Automation · ${a.enabled ? "enabled" : "disabled"}`,
      group: "Automations",
      icon: "automations",
    });
  }

  /* Areas — jump to Rooms. */
  for (const a of areas.value) {
    out.push({
      id: `area:${a.id}`,
      label: a.displayName,
      description: "Room",
      group: "Rooms",
      icon: "rooms",
    });
  }

  return out;
});

function onSelect(item: CommandItem): void {
  if (item.id.startsWith("nav:")) {
    const subject = item.id.slice("nav:".length);
    /* "settings/appearance" → "/settings/appearance"; "home" → "/" */
    if (subject === "home") router.push("/");
    else if (subject.includes("/")) router.push("/" + subject);
    else router.push("/" + subject);
    return;
  }
  if (item.id.startsWith("driver:")) {
    /* TODO once Devices supports ?driver=<id> we can deep-link.
       For now, just visit /devices. */
    router.push("/devices");
    return;
  }
  if (item.id.startsWith("automation:")) {
    router.push("/automations");
    return;
  }
  if (item.id.startsWith("area:")) {
    router.push("/rooms");
    return;
  }
}

/* Global ⌘K / Ctrl+K listener. Mounted on document so the palette
   opens regardless of focus location. */
function onKey(e: KeyboardEvent): void {
  if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
    e.preventDefault();
    paletteOpen.value = true;
  }
}

onMounted(() => {
  document.addEventListener("keydown", onKey);
  /* Eagerly fetch the catalog so the first open is instant. */
  loadCatalog();
});

onBeforeUnmount(() => {
  document.removeEventListener("keydown", onKey);
});

function onSearch(): void {
  paletteOpen.value = true;
}
</script>

<template>
  <SyShell
    :primary="PRIMARY"
    :user="{ name: 'Fynn Datoo' }"
    :active-path="route.path"
    :crumbs="crumbs"
    :daemon-status="daemonStatus"
    @navigate="onNavigate"
    @search="onSearch"
    @user-menu="onUserMenu"
  >
    <RouterView />
  </SyShell>

  <SyCommandPalette
    v-model:open="paletteOpen"
    :items="items"
    placeholder="Jump to anywhere…"
    @select="onSelect"
  />
</template>
