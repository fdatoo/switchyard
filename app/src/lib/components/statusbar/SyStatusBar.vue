<!--
  SyStatusBar — slim bottom bar for editors and developer surfaces.

  Three regions: `left`, `center`, `right`. Consumer composes content via
  slots — typically SyText caption/numeric for static info, small icons or
  dots for live status, SyKbd for shortcut hints. The bar is a flex row;
  items inside each region are spaced by gap; the three regions divide the
  bar into thirds (left and right shrink to content, center fills).

  Primary use cases per the vision spec:
    - Pkl / Starlark editor: file path, cursor position, lint, encoding,
      language version.
    - Developer language pages: RPC connection status, request latency,
      build SHA.
    - Mobile bottom info bar (when used).

  Token-driven. Surface-2 fill, line-soft top border, caption-sized
  monospace-leaning typography. Stays out of the way until you need it.
-->
<script setup lang="ts">
defineProps<{
  /** Visually de-emphasize via lower fg tone. */
  muted?: boolean;
}>();
</script>

<template>
  <div class="sy-statusbar" :class="[muted && 'sy-statusbar--muted']">
    <div v-if="$slots.left" class="sy-statusbar__region sy-statusbar__region--left">
      <slot name="left" />
    </div>
    <div v-if="$slots.center" class="sy-statusbar__region sy-statusbar__region--center">
      <slot name="center" />
    </div>
    <div v-if="$slots.right" class="sy-statusbar__region sy-statusbar__region--right">
      <slot name="right" />
    </div>
  </div>
</template>

<style scoped>
.sy-statusbar {
  display: flex;
  align-items: center;
  gap: var(--sy-space-3);
  padding: 4px var(--sy-space-3);
  background: var(--sy-color-surface-2);
  border-top: 1px solid var(--sy-color-line-soft);
  font-family: var(--sy-font-body);
  font-size: 0.6875rem;
  color: var(--sy-color-fg-3);
  min-height: 22px;
  user-select: none;
}

.sy-statusbar--muted {
  color: var(--sy-color-fg-4);
}

.sy-statusbar__region {
  display: inline-flex;
  align-items: center;
  gap: var(--sy-space-3);
  /* Visual separator between adjacent items rendered by consumers can be
     done via this `gap` plus a thin border on individual items; the bar
     itself doesn't impose dividers — keeps composition flexible. */
}

.sy-statusbar__region--left {
  flex: 0 0 auto;
  margin-right: auto;
}

.sy-statusbar__region--center {
  flex: 1 1 auto;
  justify-content: center;
  /* Center region only takes flex space if left/right aren't filling. */
}

.sy-statusbar__region--right {
  flex: 0 0 auto;
  margin-left: auto;
}
</style>
