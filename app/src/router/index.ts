import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";

import HomeView from "@/views/HomeView.vue";
import LabView from "@/views/LabView.vue";
import AppLayout from "@/views/AppLayout.vue";
import RoomsView from "@/views/RoomsView.vue";
import ActivityView from "@/views/ActivityView.vue";
import AutomationsView from "@/views/AutomationsView.vue";
import DevicesView from "@/views/DevicesView.vue";
import SettingsLayout from "@/views/settings/SettingsLayout.vue";
import AppearanceSection from "@/views/settings/sections/AppearanceSection.vue";
import AccountSection from "@/views/settings/sections/AccountSection.vue";
import DriversSection from "@/views/settings/sections/DriversSection.vue";
import SettingsStub from "@/views/settings/sections/SettingsStub.vue";

/**
 * Routes are split into two trees:
 *
 * - `/lab` and `/lab/*` — the design lab, kept independent of the shell
 *   so it can render full-bleed comparisons across language × mode cells.
 * - Everything else — wrapped in `AppLayout` which renders SyShell around
 *   the route. The shell handles all chrome (sidebar, topbar, reconnect
 *   banner); views just render their content.
 */
const routes: RouteRecordRaw[] = [
  { path: "/lab", name: "lab", component: LabView },
  {
    path: "/",
    component: AppLayout,
    children: [
      { path: "",            name: "home",        component: HomeView },
      { path: "rooms",       name: "rooms",       component: RoomsView },
      { path: "activity",    name: "activity",    component: ActivityView },
      { path: "automations", name: "automations", component: AutomationsView },
      { path: "devices",     name: "devices",     component: DevicesView },
      {
        path: "settings",
        component: SettingsLayout,
        children: [
          { path: "",             redirect: "/settings/appearance" },
          { path: "appearance",   name: "settings-appearance",   component: AppearanceSection },
          { path: "account",      name: "settings-account",      component: AccountSection },
          { path: "drivers",      name: "settings-drivers",      component: DriversSection },
          {
            path: "pkl",
            name: "settings-pkl",
            component: SettingsStub,
            props: {
              title: "Pkl config",
              icon: "developer",
              description: "An in-app editor for the daemon's Pkl configuration with live validation.",
            },
          },
          {
            path: "widget-packs",
            name: "settings-widget-packs",
            component: SettingsStub,
            props: {
              title: "Widget packs",
              icon: "rooms",
              description: "Browse and install community widget packs.",
            },
          },
          {
            path: "displays",
            name: "settings-displays",
            component: SettingsStub,
            props: {
              title: "Displays",
              icon: "devices",
              description: "Manage ambient displays and what each one shows.",
            },
          },
        ],
      },
    ],
  },
];

export const router = createRouter({
  history: createWebHistory(),
  routes,
});
