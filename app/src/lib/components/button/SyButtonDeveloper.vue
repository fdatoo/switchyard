<!--
  Developer button variant — sharp 4px corners, compact, single accent color.
  Reference points: Linear, Cursor. Internal: consumers import `SyButton`.
-->
<script setup lang="ts">
import type { ButtonIntent, ButtonSize } from "./types";

defineProps<{
  intent: ButtonIntent;
  size: ButtonSize;
  disabled?: boolean;
  type: "button" | "submit" | "reset";
}>();
</script>

<template>
  <button
    class="btn"
    :class="[`btn--${intent}`, `btn--${size}`]"
    :disabled="disabled"
    :type="type"
  >
    <slot />
  </button>
</template>

<style scoped>
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--sy-space-2);
  border-radius: var(--sy-radius);
  border: 1px solid transparent;
  font-family: var(--sy-font-body);
  font-weight: 500;
  letter-spacing: 0;
  cursor: pointer;
  transition: background var(--sy-motion-fast),
              color var(--sy-motion-fast),
              border-color var(--sy-motion-fast);
  white-space: nowrap;
}

.btn--sm { padding: 3px 8px;  font-size: 0.75rem; }
.btn--md { padding: 5px 12px; font-size: 0.8125rem; }
.btn--lg { padding: 8px 16px; font-size: 0.875rem; }

.btn--primary {
  background: var(--sy-color-accent);
  color: var(--sy-color-bg);
  border-color: var(--sy-color-accent);
}
.btn--primary:hover:not(:disabled) {
  background: var(--sy-color-accent-2);
  border-color: var(--sy-color-accent-2);
}

.btn--secondary {
  background: var(--sy-color-surface-2);
  color: var(--sy-color-fg);
  border-color: var(--sy-color-line);
}
.btn--secondary:hover:not(:disabled) {
  background: var(--sy-color-surface-3);
  border-color: var(--sy-color-accent);
}

.btn--ghost {
  background: transparent;
  color: var(--sy-color-fg-2);
  border-color: transparent;
}
.btn--ghost:hover:not(:disabled) {
  background: var(--sy-color-surface-2);
  color: var(--sy-color-fg);
  border-color: var(--sy-color-line);
}

.btn--danger {
  background: var(--sy-color-bad);
  color: var(--sy-color-bg);
  border-color: var(--sy-color-bad);
}
.btn--danger:hover:not(:disabled) {
  filter: brightness(1.1);
}

.btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.btn:focus-visible {
  outline: 1px solid var(--sy-color-accent);
  outline-offset: 1px;
}
</style>
