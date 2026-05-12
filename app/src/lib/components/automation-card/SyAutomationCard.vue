<!--
  SyAutomationCard — a single automation row.

  Composes SyListRow with structured content: an automations icon, name
  and trigger summary, a next-run hint, an enable toggle, and an overflow
  menu (`…`) with Run-now / Edit / Duplicate / Delete actions. Used in
  the Automations index page and the Automations widget on Home.

  Two trailing states:
    1. running — pulsing "Running" badge + spinner; menu and toggle still
       visible so the user can disable mid-run.
    2. idle — next-run/last-run hint + toggle + overflow menu.

  The trailing-actions container has `@click.stop.prevent` so clicks on
  any action child (menu trigger, toggle, future buttons) don't bubble to
  the row's `href` and trigger navigation. Without this, clicking the
  toggle navigates to whatever href is set on the row.

  `next-run` and `last-run` are pre-formatted strings (locale/relative-time
  rules are too varied to bake in). The component emits intent
  (`toggle-enabled` / `menu-action`); consumer wires those to RPC calls.
-->
<script setup lang="ts">
import { computed } from "vue";
import SyListRow from "@/lib/components/listrow/SyListRow.vue";
import SyText from "@/lib/components/text/SyText.vue";
import SyIcon from "@/lib/components/icon/SyIcon.vue";
import SyBadge from "@/lib/components/badge/SyBadge.vue";
import SySwitch from "@/lib/components/switch/SySwitch.vue";
import SySpinner from "@/lib/components/spinner/SySpinner.vue";
import SyMenu from "@/lib/components/menu/SyMenu.vue";
import type { MenuItem } from "@/lib/components/menu/types";

const props = withDefaults(
  defineProps<{
    name: string;
    trigger: string;
    enabled?: boolean;
    running?: boolean;
    nextRun?: string;
    lastRun?: string;
    /** Optional click target for the whole row (e.g., open detail rail). */
    href?: string;
  }>(),
  { enabled: true, running: false },
);

const emit = defineEmits<{
  "toggle-enabled": [next: boolean];
  /** Emitted when an overflow-menu item is selected. ID is one of
   *  "run" / "edit" / "duplicate" / "delete" (or any future addition). */
  "menu-action": [id: string];
}>();

const isInteractive = computed(() => Boolean(props.href));

const menuItems = computed<MenuItem[]>(() => [
  { type: "item", id: "run", label: "Run now", icon: "power", disabled: !props.enabled || props.running },
  { type: "item", id: "edit", label: "Edit", icon: "settings" },
  { type: "item", id: "duplicate", label: "Duplicate", icon: "plus" },
  { type: "separator" },
  { type: "item", id: "delete", label: "Delete", icon: "close", intent: "danger" },
]);

function onToggle(v: boolean): void {
  emit("toggle-enabled", v);
}
function onMenuSelect(id: string): void {
  emit("menu-action", id);
}
</script>

<template>
  <SyListRow
    :as="href ? 'a' : 'div'"
    :href="href"
    density="comfortable"
    :interactive="isInteractive"
    :bordered="false"
    class="sy-auto"
  >
    <template #leading>
      <span class="sy-auto__icon">
        <SyIcon name="automations" :size="18" />
      </span>
    </template>

    <SyText variant="body" weight="medium" class="sy-auto__name">
      {{ name }}
    </SyText>
    <SyText variant="caption" tone="subtle">
      {{ trigger }}
    </SyText>

    <template #trailing>
      <!-- @click.stop.prevent on the wrapper swallows clicks from any
           action child, so toggling the switch or opening the menu doesn't
           bubble to the row's href and trigger navigation. -->
      <div class="sy-auto__actions" @click.stop.prevent>
        <template v-if="running">
          <SyBadge intent="info" appearance="soft" dot pulse="fast">Running</SyBadge>
          <SySpinner :size="14" />
        </template>
        <template v-else>
          <SyText v-if="enabled && nextRun" variant="caption" tone="subtle" class="sy-auto__when">
            {{ nextRun }}
          </SyText>
          <SyText v-else-if="!enabled" variant="caption" tone="subtle">
            Disabled
          </SyText>
          <SyText v-else-if="lastRun" variant="caption" tone="subtle" class="sy-auto__when">
            Last: {{ lastRun }}
          </SyText>
        </template>

        <SySwitch :modelValue="enabled" size="sm" @update:modelValue="onToggle" />

        <SyMenu :items="menuItems" @select="onMenuSelect">
          <template #trigger>
            <button type="button" class="sy-auto__kebab" aria-label="More actions">
              <SyIcon name="more-vertical" :size="16" />
            </button>
          </template>
        </SyMenu>
      </div>
    </template>
  </SyListRow>
</template>

<style scoped>
.sy-auto :deep(.sy-listrow) {
  min-height: 60px;
}

.sy-auto__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: var(--sy-radius);
  background: var(--sy-color-accent-soft);
  color: var(--sy-color-accent);
}

.sy-auto__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sy-auto__when {
  font-family: var(--sy-font-numeric);
  font-feature-settings: var(--sy-numeric-feature);
  font-size: 0.6875rem;
}

.sy-auto__actions {
  display: inline-flex;
  align-items: center;
  gap: var(--sy-space-2);
}

.sy-auto__kebab {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  background: transparent;
  border: 0;
  border-radius: var(--sy-radius-sm);
  color: var(--sy-color-fg-3);
  cursor: pointer;
  transition: background var(--sy-motion-fast), color var(--sy-motion-fast);
}
.sy-auto__kebab:hover {
  background: var(--sy-color-surface-2);
  color: var(--sy-color-fg);
}
.sy-auto__kebab:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}
</style>
