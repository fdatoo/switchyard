<!--
  SyKbd — keyboard shortcut chip.

  Tiny rounded chip for rendering a key or combo. The text is whatever the
  consumer passes (⌘K, Ctrl+Shift+P, Esc, Enter, ↑, ↓); platform-aware
  formatting (Ctrl on Windows vs ⌘ on Mac) is the consumer's responsibility,
  not the chip's.

  Two composition styles are both valid:
    1. One chip per combo: `<SyKbd>⌘K</SyKbd>`
    2. One chip per key with a separator: `<SyKbd>⌘</SyKbd>+<SyKbd>K</SyKbd>`

  Renders an HTML `<kbd>` element so screen readers announce it as a key,
  not as plain text.
-->
<script setup lang="ts">
type Size = "sm" | "md";

withDefaults(
  defineProps<{ size?: Size }>(),
  { size: "sm" },
);
</script>

<template>
  <kbd class="sy-kbd" :class="[`sy-kbd--${size}`]">
    <slot />
  </kbd>
</template>

<style scoped>
.sy-kbd {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-family: var(--sy-font-numeric);
  font-weight: 500;
  color: var(--sy-color-fg-3);
  background: var(--sy-color-surface-2);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-sm);
  letter-spacing: 0;
  line-height: 1;
  white-space: nowrap;
}
.sy-kbd--sm {
  font-size: 0.6875rem;
  padding: 2px 5px;
  min-width: 16px;
  height: 18px;
}
.sy-kbd--md {
  font-size: 0.8125rem;
  padding: 3px 7px;
  min-width: 20px;
  height: 22px;
}
</style>
