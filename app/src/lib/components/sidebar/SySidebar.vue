<!--
  SySidebar — primary navigation rail.

  The full left-rail nav for the desktop shell. Three regions stacked
  vertically:
    1. Brand (top) — gradient square + product name, doubles as Home link.
    2. Scrollable middle — primary nav items, then a "Pages" section,
       then a "Displays" section. Empty sections render an italic
       placeholder so the structure is still visible.
    3. User pill (bottom, pinned) — avatar + name; clicking opens a
       SyMenu with account actions.

  Layout uses a single flex column with the middle region as `flex: 1
   overflow-y: auto`. This keeps the user pill flush against the bottom
  of the viewport even when nav items + sections overflow.

  Data-driven via three list props (primary, pages, displays). The
  consumer wires `navigate` to its router; the component just emits
  intent.

  Width is fixed at 200px to match the spec. The whole component is one
  vertical strip — wrap it in your shell at column 1.
-->
<script setup lang="ts">
import { computed } from "vue";
import SyText from "@/lib/components/text/SyText.vue";
import SyNavItem from "@/lib/components/navitem/SyNavItem.vue";
import SyAvatar from "@/lib/components/avatar/SyAvatar.vue";
import SyMenu from "@/lib/components/menu/SyMenu.vue";
import type { IconName } from "@/lib/components/icon/SyIcon.vue";
import type { MenuItem } from "@/lib/components/menu/types";

type BadgeIntent = "neutral" | "accent" | "good" | "warn" | "bad" | "info" | "purple";

export interface SidebarNavItem {
  id: string;
  icon: IconName;
  label: string;
  /** Full path; matched against `activePath` to decide which row is active. */
  path: string;
  /** Keyboard shortcut hint; rendered only in developer language. */
  shortcut?: string;
  /** Optional trailing badge. */
  badge?: { count: number | string; intent?: BadgeIntent };
}

export interface SidebarLink {
  id: string;
  label: string;
  path: string;
}

export interface SidebarUser {
  name: string;
  /** Avatar image URL. Falls back to initials. */
  avatarSrc?: string;
}

const props = withDefaults(
  defineProps<{
    /** Product name shown next to the brand square. */
    brand?: string;
    /** Primary nav rows (Home / Rooms / Activity / Automations / Devices / Settings). */
    primary: SidebarNavItem[];
    /** Custom user-created pages. Empty array shows the placeholder. */
    pages?: SidebarLink[];
    /** Display targets (wall tablets, fridge screens). Empty array shows placeholder. */
    displays?: SidebarLink[];
    /** Current path; used to mark a primary nav item active. */
    activePath?: string;
    /** Authenticated user. When omitted, the user pill renders a "Sign in" link. */
    user?: SidebarUser;
    /** Menu items for the user-pill dropdown. */
    userMenu?: MenuItem[];
  }>(),
  {
    brand: "Switchyard",
    pages: () => [],
    displays: () => [],
  },
);

const emit = defineEmits<{
  /** A nav item / page / display row was activated. The string is the row's path. */
  navigate: [path: string];
  /** A user-menu item was selected. The string is the menu item's `id`. */
  "user-menu": [id: string];
  /** User pill clicked while no user is signed in. */
  "sign-in": [];
}>();

function isActive(path: string): boolean {
  if (!props.activePath) return false;
  return props.activePath === path || props.activePath.startsWith(path + "/");
}

const DEFAULT_USER_MENU: MenuItem[] = [
  { type: "item", id: "account", label: "Account settings", icon: "settings" },
  { type: "item", id: "theme", label: "Theme & language", icon: "sparkle" },
  { type: "separator" },
  { type: "item", id: "signout", label: "Sign out", icon: "close" },
];

const resolvedUserMenu = computed(() => props.userMenu ?? DEFAULT_USER_MENU);
</script>

<template>
  <nav class="sy-sidebar" aria-label="Primary navigation">
    <a
      class="sy-sidebar__brand"
      href="/"
      @click.prevent="emit('navigate', '/')"
    >
      <span class="sy-sidebar__brand-square" aria-hidden="true" />
      <SyText variant="body" weight="semibold">{{ brand }}</SyText>
    </a>

    <div class="sy-sidebar__middle">
      <div class="sy-sidebar__group">
        <SyNavItem
          v-for="item in primary"
          :key="item.id"
          as="button"
          :icon="item.icon"
          :label="item.label"
          :shortcut="item.shortcut"
          :badge="item.badge"
          :active="isActive(item.path)"
          @click="emit('navigate', item.path)"
        />
      </div>

      <div class="sy-sidebar__section">
        <SyText variant="label" tone="subtle" class="sy-sidebar__sectionHead">Pages</SyText>
        <template v-if="pages.length > 0">
          <SyNavItem
            v-for="p in pages"
            :key="p.id"
            as="button"
            :label="p.label"
            :active="isActive(p.path)"
            @click="emit('navigate', p.path)"
          />
        </template>
        <SyText v-else variant="caption" tone="subtle" class="sy-sidebar__empty">
          No custom pages yet.
        </SyText>
      </div>

      <div class="sy-sidebar__section">
        <SyText variant="label" tone="subtle" class="sy-sidebar__sectionHead">Displays</SyText>
        <template v-if="displays.length > 0">
          <SyNavItem
            v-for="d in displays"
            :key="d.id"
            as="button"
            :label="d.label"
            :active="isActive(d.path)"
            @click="emit('navigate', d.path)"
          />
        </template>
        <SyText v-else variant="caption" tone="subtle" class="sy-sidebar__empty">
          No displays yet.
        </SyText>
      </div>
    </div>

    <div class="sy-sidebar__user">
      <SyMenu v-if="user" :items="resolvedUserMenu" @select="(id) => emit('user-menu', id)">
        <template #trigger>
          <button type="button" class="sy-sidebar__userPill">
            <SyAvatar :name="user.name" :src="user.avatarSrc" size="sm" />
            <SyText variant="caption" weight="medium" class="sy-sidebar__userName">
              {{ user.name }}
            </SyText>
          </button>
        </template>
      </SyMenu>
      <button v-else type="button" class="sy-sidebar__signin" @click="emit('sign-in')">
        <SyAvatar size="sm" />
        <SyText variant="caption" weight="medium" tone="accent">Sign in</SyText>
      </button>
    </div>
  </nav>
</template>

<style scoped>
.sy-sidebar {
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  width: 200px;
  height: 100vh;
  max-height: 100vh;
  padding: 14px 10px;
  background: var(--sy-color-sidebar);
  border-right: 1px solid var(--sy-color-line);
  flex-shrink: 0;
  overflow: hidden;
}

.sy-sidebar__brand {
  display: flex;
  align-items: center;
  gap: var(--sy-space-2);
  padding: 4px 8px 14px;
  text-decoration: none;
  color: var(--sy-color-fg);
  flex-shrink: 0;
}
.sy-sidebar__brand-square {
  width: 22px;
  height: 22px;
  border-radius: 7px;
  background: linear-gradient(135deg, var(--sy-color-accent), var(--sy-color-accent-2));
  box-shadow: var(--sy-shadow);
  flex-shrink: 0;
}

.sy-sidebar__middle {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-3);
  /* Hide native scrollbar visual fluff; it shows on hover only via the
     browser's overlay behavior on macOS / Windows 11. */
}

.sy-sidebar__group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.sy-sidebar__section {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.sy-sidebar__sectionHead {
  padding: 0 10px 4px;
}
.sy-sidebar__empty {
  padding: 2px 10px 6px;
  font-style: italic;
}

.sy-sidebar__user {
  flex-shrink: 0;
  padding-top: var(--sy-space-2);
  border-top: 1px solid var(--sy-color-line-soft);
}

.sy-sidebar__userPill,
.sy-sidebar__signin {
  display: flex;
  align-items: center;
  gap: var(--sy-space-2);
  width: 100%;
  padding: 6px 8px;
  background: transparent;
  border: 0;
  border-radius: var(--sy-radius);
  cursor: pointer;
  text-align: left;
  font: inherit;
  color: inherit;
  transition: background var(--sy-motion-fast);
}
.sy-sidebar__userPill:hover,
.sy-sidebar__signin:hover {
  background: var(--sy-color-surface-2);
}
.sy-sidebar__userPill:focus-visible,
.sy-sidebar__signin:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}

.sy-sidebar__userName {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
