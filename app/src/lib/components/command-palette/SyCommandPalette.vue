<!--
  SyCommandPalette — Spotlight-style searchable launcher.

  Dumb component: parent passes `open` + `items` and listens for
  `update:open` and `select`. The palette owns its query state and
  keyboard navigation (arrows, Enter, Esc).

  Items are grouped by `group` field; groups render in the order they
  first appear in the items list — the parent controls ordering by
  ordering items appropriately.

  Visual: centered glass card with a search input on top and a
  scrollable result list below. ⌘K / Ctrl+K listener lives in the
  parent, not here, so multiple palettes don't fight over the shortcut.

  Trade-offs:
  - Substring (not fuzzy) match keeps the implementation tiny; for the
    catalog sizes we expect (~hundreds of items) it's plenty fast and
    feels deterministic to the user.
  - Categories are visual headers, not selectable.
-->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import SyIcon, { type IconName } from "@/lib/components/icon/SyIcon.vue";
import SyText from "@/lib/components/text/SyText.vue";
import SyKbd from "@/lib/components/kbd/SyKbd.vue";

export interface CommandItem {
  /** Stable identifier; emitted in `select`. */
  id: string;
  /** Visible label, also matched against query. */
  label: string;
  /** Optional secondary description (matched against query too). */
  description?: string;
  /** Group heading the item belongs to. */
  group: string;
  /** Leading icon. */
  icon: IconName;
  /** Optional badge or keyboard shortcut shown trailing. */
  shortcut?: string;
}

const props = withDefaults(
  defineProps<{
    open: boolean;
    items: CommandItem[];
    /** Placeholder for the search input. */
    placeholder?: string;
  }>(),
  { placeholder: "Search anything…" },
);

const emit = defineEmits<{
  "update:open": [open: boolean];
  select: [item: CommandItem];
}>();

const query = ref<string>("");
const activeIndex = ref<number>(0);
const inputEl = ref<HTMLInputElement | null>(null);
const listEl = ref<HTMLElement | null>(null);

const needle = computed<string>(() => query.value.trim().toLowerCase());

const filteredItems = computed<CommandItem[]>(() => {
  if (!needle.value) return props.items;
  return props.items.filter((it) => {
    if (it.label.toLowerCase().includes(needle.value)) return true;
    if (it.description?.toLowerCase().includes(needle.value)) return true;
    return false;
  });
});

/**
 * Group the filtered list by `group`, preserving first-appearance
 * order. Returns a flat array alternating group headers and items so
 * the template can render with a single v-for.
 */
interface GroupHeader { kind: "header"; group: string; }
interface ItemRow    { kind: "item"; item: CommandItem; index: number; }
type Row = GroupHeader | ItemRow;

const rows = computed<Row[]>(() => {
  const out: Row[] = [];
  let lastGroup = "";
  let i = 0;
  for (const it of filteredItems.value) {
    if (it.group !== lastGroup) {
      out.push({ kind: "header", group: it.group });
      lastGroup = it.group;
    }
    out.push({ kind: "item", item: it, index: i });
    i++;
  }
  return out;
});

/** Total number of selectable items (excludes headers). */
const itemCount = computed<number>(() => filteredItems.value.length);

watch(() => props.open, async (open) => {
  if (open) {
    query.value = "";
    activeIndex.value = 0;
    await nextTick();
    inputEl.value?.focus();
  }
});

/* Reset selection when the filter changes (keeps the cursor visible). */
watch(filteredItems, () => { activeIndex.value = 0; });

function close(): void {
  emit("update:open", false);
}

function selectActive(): void {
  const it = filteredItems.value[activeIndex.value];
  if (!it) return;
  emit("select", it);
  close();
}

function onSelect(item: CommandItem): void {
  emit("select", item);
  close();
}

function onKey(e: KeyboardEvent): void {
  if (e.key === "Escape") { e.preventDefault(); close(); return; }
  if (e.key === "Enter")  { e.preventDefault(); selectActive(); return; }
  if (e.key === "ArrowDown" || (e.key === "n" && e.ctrlKey)) {
    e.preventDefault();
    activeIndex.value = Math.min(itemCount.value - 1, activeIndex.value + 1);
    scrollActiveIntoView();
    return;
  }
  if (e.key === "ArrowUp" || (e.key === "p" && e.ctrlKey)) {
    e.preventDefault();
    activeIndex.value = Math.max(0, activeIndex.value - 1);
    scrollActiveIntoView();
    return;
  }
}

/** Keep the active item in view as the user arrows past the visible
    edges. We rely on the DOM order matching `filteredItems` order. */
function scrollActiveIntoView(): void {
  const list = listEl.value;
  if (!list) return;
  const el = list.querySelector<HTMLElement>(`[data-row="${activeIndex.value}"]`);
  if (el) el.scrollIntoView({ block: "nearest" });
}
</script>

<template>
  <Transition name="sy-cmd">
    <div v-if="open" class="sy-cmd" @keydown="onKey">
      <div class="sy-cmd__backdrop" @click="close" />

      <div class="sy-cmd__dialog" role="dialog" aria-label="Command palette">
        <div class="sy-cmd__inputRow">
          <SyIcon name="search" :size="18" class="sy-cmd__searchIcon" />
          <input
            ref="inputEl"
            v-model="query"
            type="text"
            class="sy-cmd__input"
            :placeholder="placeholder"
            autocomplete="off"
            spellcheck="false"
          />
          <SyKbd>esc</SyKbd>
        </div>

        <div ref="listEl" class="sy-cmd__list" role="listbox">
          <template v-for="(row, i) in rows" :key="i">
            <SyText
              v-if="row.kind === 'header'"
              variant="overline"
              tone="subtle"
              weight="medium"
              class="sy-cmd__group"
            >
              {{ row.group }}
            </SyText>
            <button
              v-else
              type="button"
              role="option"
              :aria-selected="activeIndex === row.index"
              :data-row="row.index"
              :class="[
                'sy-cmd__row',
                activeIndex === row.index && 'sy-cmd__row--active',
              ]"
              @click="onSelect(row.item)"
              @mouseenter="activeIndex = row.index"
            >
              <span class="sy-cmd__rowIcon"><SyIcon :name="row.item.icon" :size="16" /></span>
              <span class="sy-cmd__rowText">
                <span class="sy-cmd__rowLabel">{{ row.item.label }}</span>
                <span v-if="row.item.description" class="sy-cmd__rowDesc">{{ row.item.description }}</span>
              </span>
              <SyKbd v-if="row.item.shortcut">{{ row.item.shortcut }}</SyKbd>
            </button>
          </template>

          <div v-if="filteredItems.length === 0" class="sy-cmd__empty">
            <SyText variant="caption" tone="subtle">
              No matches for "{{ query }}".
            </SyText>
          </div>
        </div>

        <footer class="sy-cmd__footer">
          <span class="sy-cmd__hint">
            <SyKbd>↑</SyKbd><SyKbd>↓</SyKbd>
            <SyText variant="caption" tone="subtle">navigate</SyText>
          </span>
          <span class="sy-cmd__hint">
            <SyKbd>⏎</SyKbd>
            <SyText variant="caption" tone="subtle">select</SyText>
          </span>
          <span class="sy-cmd__hint">
            <SyKbd>esc</SyKbd>
            <SyText variant="caption" tone="subtle">close</SyText>
          </span>
        </footer>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.sy-cmd {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 10vh;
}
.sy-cmd__backdrop {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.32);
  -webkit-backdrop-filter: blur(2px);
  backdrop-filter: blur(2px);
}
.sy-cmd__dialog {
  position: relative;
  width: min(680px, calc(100% - 32px));
  background: var(--sy-color-surface-1);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-lg);
  box-shadow: var(--sy-shadow-large, 0 24px 64px rgba(0, 0, 0, 0.2));
  display: flex;
  flex-direction: column;
  max-height: 70vh;
  overflow: hidden;
}

.sy-cmd__inputRow {
  display: flex;
  align-items: center;
  gap: var(--sy-space-2);
  padding: var(--sy-space-3) var(--sy-space-4);
  border-bottom: 1px solid var(--sy-color-line-soft);
}
.sy-cmd__searchIcon {
  color: var(--sy-color-fg-3);
  flex-shrink: 0;
}
.sy-cmd__input {
  flex: 1;
  background: transparent;
  border: 0;
  outline: 0;
  font: inherit;
  font-size: 1rem;
  color: var(--sy-color-fg);
}
.sy-cmd__input::placeholder { color: var(--sy-color-fg-3); }

.sy-cmd__list {
  flex: 1;
  overflow-y: auto;
  padding: var(--sy-space-2) 0;
}

.sy-cmd__group {
  display: block;
  padding: var(--sy-space-3) var(--sy-space-4) var(--sy-space-1);
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.sy-cmd__row {
  display: flex;
  align-items: center;
  gap: var(--sy-space-3);
  width: 100%;
  padding: var(--sy-space-2) var(--sy-space-4);
  background: transparent;
  border: 0;
  cursor: pointer;
  text-align: left;
  color: inherit;
  font: inherit;
  transition: background var(--sy-motion-fast);
}
.sy-cmd__row--active {
  background: var(--sy-color-surface-2);
}
.sy-cmd__rowIcon {
  display: inline-flex;
  align-items: center;
  color: var(--sy-color-fg-3);
  flex-shrink: 0;
}
.sy-cmd__rowText {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  flex: 1;
}
.sy-cmd__rowLabel { font-weight: 500; }
.sy-cmd__rowDesc {
  color: var(--sy-color-fg-3);
  font-size: 0.78rem;
}

.sy-cmd__empty {
  padding: var(--sy-space-4);
  text-align: center;
}

.sy-cmd__footer {
  display: flex;
  gap: var(--sy-space-4);
  align-items: center;
  padding: var(--sy-space-2) var(--sy-space-4);
  border-top: 1px solid var(--sy-color-line-soft);
  background: var(--sy-color-surface-2);
}
.sy-cmd__hint {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

/* Smooth open/close. Quick ease-out cubic — feels native. */
.sy-cmd-enter-active, .sy-cmd-leave-active {
  transition: opacity 120ms ease-out;
}
.sy-cmd-enter-active .sy-cmd__dialog,
.sy-cmd-leave-active .sy-cmd__dialog {
  transition: transform 160ms cubic-bezier(0.16, 1, 0.3, 1), opacity 160ms ease-out;
}
.sy-cmd-enter-from { opacity: 0; }
.sy-cmd-leave-to   { opacity: 0; }
.sy-cmd-enter-from .sy-cmd__dialog {
  transform: translateY(-12px);
  opacity: 0;
}
.sy-cmd-leave-to .sy-cmd__dialog {
  transform: translateY(-6px);
  opacity: 0;
}
</style>
