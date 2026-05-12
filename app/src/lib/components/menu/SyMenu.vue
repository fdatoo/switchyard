<!--
  SyMenu — dropdown menu.

  The canonical "click a trigger, pick from a list of actions" primitive.
  Backs the user-pill dropdown, the overflow `…` button on list rows, the
  filter chips on the Activity page, etc. Right-click context menus reuse
  the same surface; a future `trigger="contextmenu"` mode will support that.

  Data-driven: pass a `MenuItem[]` array. Three item shapes (action,
  separator, header) cover the typical menu structure without nested
  sub-arrays. The item type is a discriminated union so TypeScript narrows
  per-branch.

  Trigger: provide via the `trigger` slot. The wrapping `<span>` listens
  for clicks and toggles open. The slot can receive `{ isOpen }` via
  scoped slot if the trigger needs to render state (e.g., flipping a
  chevron icon).

  Positioning: teleported to body with `position: fixed`. Default placement
  is below-left-aligned with the trigger; flips above if it would overflow
  viewport bottom. Horizontal position is clamped so menus near the right
  edge stay on-screen.

  Theme inheritance: when the trigger lives inside a per-region theme
  scope (e.g., a /lab cell with `data-language="ambient"`), the teleported
  menu would normally inherit only the document-level theme — wrong. We
  walk up from the anchor to find the effective `data-language` / `data-mode`
  and apply them to the menu element, so the menu's tokens match the
  trigger's region.

  Dismiss: clicking outside closes; Escape closes. The outside-click
  listener is attached only while open to avoid global event noise.
-->
<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from "vue";
import SyIcon from "@/lib/components/icon/SyIcon.vue";
import SyKbd from "@/lib/components/kbd/SyKbd.vue";
import { applyAnchorTheme } from "@/lib/theme/inherit-theme";
import type { MenuItem, MenuItemAction } from "./types";

const props = defineProps<{
  items: MenuItem[];
  /** Min-width of the menu in px. Defaults to the trigger's width. */
  minWidth?: number;
}>();

const emit = defineEmits<{
  select: [id: string];
}>();

const anchorRef = ref<HTMLElement | null>(null);
const menuRef = ref<HTMLElement | null>(null);
const open = ref(false);
const menuStyle = ref<Record<string, string>>({});

function toggle(): void {
  if (open.value) close();
  else show();
}

function show(): void {
  open.value = true;
  nextTick(() => {
    /* Apply the trigger's effective theme to the teleported menu so it
       picks up the right tokens (per-region theme scopes like /lab cells
       wouldn't reach it otherwise). */
    if (menuRef.value) applyAnchorTheme(menuRef.value, anchorRef.value);
    updatePosition();
    document.addEventListener("mousedown", onOutsideClick);
    document.addEventListener("keydown", onKey);
  });
}

function close(): void {
  open.value = false;
  document.removeEventListener("mousedown", onOutsideClick);
  document.removeEventListener("keydown", onKey);
}

function onOutsideClick(e: MouseEvent): void {
  const target = e.target as Node;
  if (anchorRef.value?.contains(target)) return;
  if (menuRef.value?.contains(target)) return;
  close();
}

function onKey(e: KeyboardEvent): void {
  if (e.key === "Escape") close();
}

function select(item: MenuItemAction): void {
  if (item.disabled) return;
  emit("select", item.id);
  close();
}

/**
 * Place the menu below the trigger by default. If it would overflow the
 * viewport bottom, flip above. Horizontal position is clamped so menus
 * near the right edge stay fully on-screen. Min-width defaults to the
 * trigger's own width — keeps dropdowns aligned with their button.
 */
function updatePosition(): void {
  const anchor = anchorRef.value;
  const menu = menuRef.value;
  if (!anchor || !menu) return;

  const a = anchor.getBoundingClientRect();
  const m = menu.getBoundingClientRect();
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const gap = 4;
  const pad = 8;

  const flipAbove = a.bottom + m.height + gap > vh - pad;
  const top = flipAbove ? a.top - m.height - gap : a.bottom + gap;
  let left = a.left;
  left = Math.max(pad, Math.min(left, vw - m.width - pad));

  menuStyle.value = {
    top: `${top}px`,
    left: `${left}px`,
    minWidth: `${props.minWidth ?? a.width}px`,
  };
}

watch(open, (isOpen) => {
  if (isOpen) nextTick(updatePosition);
});

onUnmounted(() => {
  document.removeEventListener("mousedown", onOutsideClick);
  document.removeEventListener("keydown", onKey);
});

defineExpose({ open, show, close });

const triggerSlotProps = computed(() => ({ isOpen: open.value }));
</script>

<template>
  <span ref="anchorRef" class="sy-menu__anchor" @click="toggle">
    <slot name="trigger" v-bind="triggerSlotProps" />
  </span>
  <Teleport to="body">
    <Transition name="sy-menu">
      <div
        v-if="open"
        ref="menuRef"
        class="sy-menu"
        :style="menuStyle"
        role="menu"
      >
        <template v-for="(item, i) in items" :key="i">
          <hr v-if="item.type === 'separator'" class="sy-menu__sep" aria-hidden="true" />
          <div v-else-if="item.type === 'header'" class="sy-menu__header">
            {{ item.label }}
          </div>
          <button
            v-else
            type="button"
            class="sy-menu__item"
            :class="[item.intent === 'danger' && 'sy-menu__item--danger']"
            :disabled="item.disabled"
            role="menuitem"
            @click="select(item)"
          >
            <span class="sy-menu__icon">
              <SyIcon v-if="item.icon" :name="item.icon" :size="14" />
            </span>
            <span class="sy-menu__label">{{ item.label }}</span>
            <SyKbd v-if="item.shortcut" class="sy-menu__shortcut">{{ item.shortcut }}</SyKbd>
          </button>
        </template>
      </div>
    </Transition>
  </Teleport>
</template>

<style>
/* Unscoped — teleport target (body) is outside this component's scope.
   Class names are namespaced (`sy-menu*`). */

.sy-menu {
  position: fixed;
  z-index: 110;
  background: var(--sy-color-surface-1);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius);
  box-shadow: var(--sy-shadow-elevated);
  padding: var(--sy-space-1);
  min-width: 180px;
  max-height: 60vh;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.sy-menu__item {
  display: grid;
  grid-template-columns: 18px 1fr auto;
  gap: var(--sy-space-2);
  align-items: center;
  padding: 6px 8px;
  background: transparent;
  border: 0;
  border-radius: var(--sy-radius-sm);
  color: var(--sy-color-fg);
  font-family: var(--sy-font-body);
  font-size: 0.8125rem;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
  width: 100%;
  transition: background var(--sy-motion-fast), color var(--sy-motion-fast);
}

.sy-menu__item:hover:not(:disabled) {
  background: var(--sy-color-surface-2);
}
.sy-menu__item:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: -2px;
}
.sy-menu__item:disabled {
  color: var(--sy-color-fg-4);
  cursor: not-allowed;
}

.sy-menu__item--danger {
  color: var(--sy-color-bad);
}
.sy-menu__item--danger:hover:not(:disabled) {
  background: color-mix(in srgb, var(--sy-color-bad) 10%, transparent);
}

.sy-menu__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--sy-color-fg-3);
}
.sy-menu__item--danger .sy-menu__icon {
  color: var(--sy-color-bad);
}

.sy-menu__label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sy-menu__shortcut {
  margin-left: var(--sy-space-2);
}

.sy-menu__sep {
  margin: 4px 0;
  border: 0;
  border-top: 1px solid var(--sy-color-line-soft);
}

.sy-menu__header {
  padding: 6px 8px 2px;
  font-family: var(--sy-font-body);
  font-size: 0.6875rem;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--sy-color-fg-4);
}

.sy-menu-enter-active,
.sy-menu-leave-active {
  transition: opacity 120ms ease, transform 140ms cubic-bezier(0.16, 1, 0.3, 1);
}
.sy-menu-enter-from,
.sy-menu-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

/* Both selectors needed: the descendant form covers the case where ambient
   is set on an ancestor (e.g., body or html), the self-attr form covers
   the case where `applyAnchorTheme` writes `data-language` onto the menu
   itself. Without the self-attr selector, the generic
   `[data-language="ambient"]` rule in `tokens/index.css` matches the menu
   directly and paints it with the page gradient — the menu becomes
   transparent over half the screen and shows the light page through. */
[data-language="ambient"] .sy-menu,
.sy-menu[data-language="ambient"] {
  /* Ambient's `surface-1` is a 6%-white tint meant to sit over the page's
     gradient — too faint for a floating menu against a dark canvas. We go
     the OTHER way: a darker overlay over the canvas creates a "pressed-in
     glass" feel that visibly recedes from the page. A bright rim + deep
     shadow + inset top highlight together frame it as elevated chrome. */
  background:
    linear-gradient(rgba(0, 0, 0, 0.35), rgba(0, 0, 0, 0.5)),
    var(--sy-color-bg);
  border-color: rgba(255, 255, 255, 0.18);
  box-shadow:
    0 12px 32px rgba(0, 0, 0, 0.55),
    inset 0 1px 0 rgba(255, 255, 255, 0.08);
  -webkit-backdrop-filter: blur(20px) saturate(140%);
  backdrop-filter: blur(20px) saturate(140%);
}
</style>

<style scoped>
.sy-menu__anchor {
  display: inline-flex;
}
</style>
