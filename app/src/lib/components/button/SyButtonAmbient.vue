<!--
  Ambient button variant — glassmorphic capsule, 44px+ touch target, backdrop-
  blur + saturate. Reference points: Apple Home, Things. Internal: consumers
  import `SyButton`.
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
  border-radius: var(--sy-radius-xl);
  border: 1px solid var(--sy-color-line);
  font-family: var(--sy-font-body);
  font-weight: 500;
  letter-spacing: -0.005em;
  cursor: pointer;
  -webkit-backdrop-filter: blur(20px) saturate(140%);
  backdrop-filter: blur(20px) saturate(140%);
  transition: background var(--sy-motion),
              color var(--sy-motion),
              border-color var(--sy-motion),
              transform var(--sy-motion-fast);
  white-space: nowrap;
}

.btn--sm { padding: 10px 18px; font-size: 0.875rem;  min-height: 36px; }
.btn--md { padding: 14px 24px; font-size: 0.9375rem; min-height: 44px; }
.btn--lg { padding: 18px 32px; font-size: 1.0625rem; min-height: 56px; }

.btn--primary {
  background: var(--sy-color-accent-soft);
  color: var(--sy-color-fg);
  border-color: var(--sy-color-accent);
}
.btn--primary:hover:not(:disabled) {
  background: var(--sy-color-accent);
  color: #fff;
}

.btn--secondary {
  background: var(--sy-color-surface-1);
  color: var(--sy-color-fg);
}
.btn--secondary:hover:not(:disabled) {
  background: var(--sy-color-surface-2);
}

.btn--ghost {
  background: transparent;
  color: var(--sy-color-fg-2);
  border-color: transparent;
}
.btn--ghost:hover:not(:disabled) {
  background: var(--sy-color-surface-1);
  color: var(--sy-color-fg);
}

.btn--danger {
  background: rgba(232, 122, 95, 0.18);
  color: var(--sy-color-fg);
  border-color: var(--sy-color-bad);
}
.btn--danger:hover:not(:disabled) {
  background: var(--sy-color-bad);
  color: #fff;
}

.btn:active:not(:disabled) {
  transform: scale(0.97);
}
.btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.btn:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 3px;
}
</style>
