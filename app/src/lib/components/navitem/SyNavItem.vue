<!--
  SyNavItem — sidebar navigation row.

  Composes SyIcon + label + optional badge + optional keyboard shortcut.
  Three states: default, hover, active. Active is the meaningful one — it
  lifts the row with `surface-1` background, shadow, and tints the icon
  with the accent color.

  Keyboard shortcut chip is shown only when the active language is
  `developer` (matching the C10 convention — developer mode surfaces
  shortcuts globally, friendly/ambient surface them via the command
  palette or tooltips). Scoped via the `[data-language="developer"]`
  descendant rule, no JS.

  Defaults to `<a>` for navigation. Use `as="button"` for action-only nav
  items (rare in the sidebar; common in dropdown menus).

  ARIA: `aria-current="page"` on the active row; `aria-disabled` when
  disabled (in addition to dimming).
-->
<script setup lang="ts">
import { computed } from "vue";
import SyIcon, { type IconName } from "@/lib/components/icon/SyIcon.vue";
import SyKbd from "@/lib/components/kbd/SyKbd.vue";
import SyBadge from "@/lib/components/badge/SyBadge.vue";

type AsTag = "a" | "button" | "div";
type BadgeIntent = "neutral" | "accent" | "good" | "warn" | "bad" | "info" | "purple";

const props = withDefaults(
  defineProps<{
    /** Leading icon. */
    icon?: IconName;
    /** Visible label. */
    label: string;
    /** Keyboard shortcut text. Rendered only in developer language. */
    shortcut?: string;
    /** Optional trailing notification badge. */
    badge?: { count: number | string; intent?: BadgeIntent };
    /** Currently-selected row. Adds `aria-current="page"`. */
    active?: boolean;
    disabled?: boolean;
    as?: AsTag;
    href?: string;
  }>(),
  { as: "a", active: false, disabled: false },
);

const classes = computed(() => [
  "sy-navitem",
  props.active && "sy-navitem--active",
  props.disabled && "sy-navitem--disabled",
]);
</script>

<template>
  <component
    :is="as"
    :href="as === 'a' && !disabled ? href : undefined"
    :class="classes"
    :aria-current="active ? 'page' : undefined"
    :aria-disabled="disabled || undefined"
    :disabled="as === 'button' && disabled ? true : undefined"
  >
    <span v-if="icon" class="sy-navitem__icon">
      <SyIcon :name="icon" :size="18" />
    </span>
    <span class="sy-navitem__label">{{ label }}</span>
    <SyBadge
      v-if="badge"
      size="sm"
      :intent="badge.intent ?? 'neutral'"
      class="sy-navitem__badge"
    >
      {{ badge.count }}
    </SyBadge>
    <SyKbd v-if="shortcut" class="sy-navitem__kbd">{{ shortcut }}</SyKbd>
  </component>
</template>

<style scoped>
.sy-navitem {
  display: flex;
  align-items: center;
  gap: var(--sy-space-2);
  padding: 7px 10px;
  border-radius: var(--sy-radius);
  color: var(--sy-color-fg-2);
  background: transparent;
  border: 0;
  font-family: var(--sy-font-body);
  font-size: 0.8125rem;
  font-weight: 500;
  text-decoration: none;
  cursor: pointer;
  width: 100%;
  text-align: left;
  transition: background var(--sy-motion-fast),
              color var(--sy-motion-fast),
              box-shadow var(--sy-motion-fast);
}

.sy-navitem__icon {
  display: inline-flex;
  align-items: center;
  color: var(--sy-color-fg-3);
  flex-shrink: 0;
  transition: color var(--sy-motion-fast);
}

.sy-navitem__label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sy-navitem__badge {
  flex-shrink: 0;
}

/* Shortcut chip is hidden by default. Developer language activates it
   globally via the descendant selector below — same convention as C10. */
.sy-navitem__kbd {
  display: none;
  flex-shrink: 0;
}
[data-language="developer"] .sy-navitem__kbd {
  display: inline-flex;
}

.sy-navitem:hover:not(.sy-navitem--active):not(.sy-navitem--disabled) {
  background: var(--sy-color-surface-2);
  color: var(--sy-color-fg);
}

.sy-navitem--active {
  background: var(--sy-color-surface-1);
  color: var(--sy-color-fg);
  box-shadow: var(--sy-shadow);
}
.sy-navitem--active .sy-navitem__icon {
  color: var(--sy-color-accent);
}

.sy-navitem--disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.sy-navitem:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}

/* Ambient sidebars are glassy. */
[data-language="ambient"] .sy-navitem--active {
  -webkit-backdrop-filter: blur(20px) saturate(140%);
  backdrop-filter: blur(20px) saturate(140%);
}
</style>
