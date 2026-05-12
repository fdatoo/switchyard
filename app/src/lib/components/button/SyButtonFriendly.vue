<!--
  Friendly button variant — pill shape, soft accent, generous padding.
  Reference points: Tower, Tailscale, Stripe Dashboard. Internal: consumers
  import `SyButton` (the dispatcher), not this file.
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
  border-radius: var(--sy-radius-pill);
  border: 1px solid transparent;
  font-family: var(--sy-font-body);
  font-weight: 500;
  letter-spacing: -0.005em;
  cursor: pointer;
  transition: background var(--sy-motion-fast),
              color var(--sy-motion-fast),
              border-color var(--sy-motion-fast),
              box-shadow var(--sy-motion-fast);
  white-space: nowrap;
}

.btn--sm { padding: 5px 12px; font-size: 0.8125rem; }
.btn--md { padding: 7px 16px; font-size: 0.875rem; }
.btn--lg { padding: 10px 22px; font-size: 0.9375rem; }

.btn--primary {
  background: var(--sy-color-accent);
  color: #fff;
  box-shadow: var(--sy-shadow);
}
.btn--primary:hover:not(:disabled) {
  background: var(--sy-color-accent-2);
}

.btn--secondary {
  background: var(--sy-color-surface-1);
  color: var(--sy-color-fg);
  border-color: var(--sy-color-line);
  box-shadow: var(--sy-shadow);
}
.btn--secondary:hover:not(:disabled) {
  background: var(--sy-color-surface-2);
}

.btn--ghost {
  background: transparent;
  color: var(--sy-color-fg-2);
}
.btn--ghost:hover:not(:disabled) {
  background: var(--sy-color-surface-2);
  color: var(--sy-color-fg);
}

.btn--danger {
  background: var(--sy-color-bad);
  color: #fff;
  box-shadow: var(--sy-shadow);
}
.btn--danger:hover:not(:disabled) {
  filter: brightness(1.05);
}

.btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.btn:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}
</style>
