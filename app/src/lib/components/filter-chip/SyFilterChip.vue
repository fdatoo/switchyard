<!--
  SyFilterChip — removable filter pill.

  Represents one active filter: "Status: Running ×". Click the body to
  edit/toggle (emits `click`, typically opens a SyMenu); click the × to
  remove (emits `remove`). Two-button structure with a 1px divider
  inherits the canonical Linear/Jira/Notion filter-chip pattern.

  Intent tints the chip via `color-mix()` against the intent's base color
  — no new tokens needed for soft variants of every intent.
-->
<script setup lang="ts">
import SyIcon from "@/lib/components/icon/SyIcon.vue";

withDefaults(
  defineProps<{
    /** Optional field name, rendered as "Field:" before the label. */
    field?: string;
    /** Filter value or summary. */
    label: string;
    /** Hide the remove button if false. */
    removable?: boolean;
    /** Tint the chip with a semantic color. */
    intent?: "neutral" | "accent" | "good" | "warn" | "bad";
  }>(),
  { removable: true, intent: "neutral" },
);

const emit = defineEmits<{
  remove: [];
  click: [];
}>();
</script>

<template>
  <span class="sy-fchip" :class="[`sy-fchip--intent-${intent}`]">
    <button type="button" class="sy-fchip__body" @click="emit('click')">
      <span v-if="field" class="sy-fchip__field">{{ field }}:</span>
      <span class="sy-fchip__label">{{ label }}</span>
    </button>
    <button
      v-if="removable"
      type="button"
      class="sy-fchip__close"
      :aria-label="`Remove filter ${field ?? ''} ${label}`.trim()"
      @click="emit('remove')"
    >
      <SyIcon name="close" :size="10" />
    </button>
  </span>
</template>

<style scoped>
.sy-fchip {
  display: inline-flex;
  align-items: stretch;
  background: var(--sy-color-surface-2);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-pill);
  font-family: var(--sy-font-body);
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--sy-color-fg);
  overflow: hidden;
  height: 22px;
}

.sy-fchip__body {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 0 8px;
  background: transparent;
  border: 0;
  color: inherit;
  font: inherit;
  cursor: pointer;
  transition: background var(--sy-motion-fast);
}
.sy-fchip__body:hover {
  background: color-mix(in srgb, currentColor 8%, transparent);
}

.sy-fchip__field {
  color: var(--sy-color-fg-3);
}

.sy-fchip__close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  background: transparent;
  border: 0;
  border-left: 1px solid var(--sy-color-line);
  color: var(--sy-color-fg-4);
  cursor: pointer;
  transition: background var(--sy-motion-fast), color var(--sy-motion-fast);
}
.sy-fchip__close:hover {
  background: color-mix(in srgb, var(--sy-color-bad) 14%, transparent);
  color: var(--sy-color-bad);
}

/* Intent tints — derived via color-mix from a single intent color. */
.sy-fchip--intent-accent {
  background: color-mix(in srgb, var(--sy-color-accent) 12%, transparent);
  border-color: color-mix(in srgb, var(--sy-color-accent) 40%, transparent);
  color: var(--sy-color-accent);
}
.sy-fchip--intent-accent .sy-fchip__field {
  color: color-mix(in srgb, var(--sy-color-accent) 60%, var(--sy-color-fg-3));
}
.sy-fchip--intent-accent .sy-fchip__close {
  border-left-color: color-mix(in srgb, var(--sy-color-accent) 40%, transparent);
  color: color-mix(in srgb, var(--sy-color-accent) 75%, var(--sy-color-fg-3));
}

.sy-fchip--intent-good {
  background: color-mix(in srgb, var(--sy-color-good) 12%, transparent);
  border-color: color-mix(in srgb, var(--sy-color-good) 40%, transparent);
  color: var(--sy-color-good);
}
.sy-fchip--intent-good .sy-fchip__close {
  border-left-color: color-mix(in srgb, var(--sy-color-good) 40%, transparent);
}
.sy-fchip--intent-warn {
  background: color-mix(in srgb, var(--sy-color-warn) 12%, transparent);
  border-color: color-mix(in srgb, var(--sy-color-warn) 40%, transparent);
  color: var(--sy-color-warn);
}
.sy-fchip--intent-warn .sy-fchip__close {
  border-left-color: color-mix(in srgb, var(--sy-color-warn) 40%, transparent);
}
.sy-fchip--intent-bad {
  background: color-mix(in srgb, var(--sy-color-bad) 12%, transparent);
  border-color: color-mix(in srgb, var(--sy-color-bad) 40%, transparent);
  color: var(--sy-color-bad);
}
.sy-fchip--intent-bad .sy-fchip__close {
  border-left-color: color-mix(in srgb, var(--sy-color-bad) 40%, transparent);
}
</style>
