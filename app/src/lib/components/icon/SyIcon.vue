<!--
  SyIcon — the single icon primitive.

  All built-in icons live in this one SFC, keyed by `name`. The set is small
  (~17 icons, a few KB of inline SVG) so the bundler-tree-shaking we'd gain
  by splitting per-icon isn't worth the file-explosion. If the icon set
  grows past ~50, revisit.

  All icons share the same geometry contract: 24×24 viewBox, 1.6px stroke,
  no fill, `currentColor` stroke + fill. Color flows in via the parent's
  CSS `color` property — set the parent's tone (e.g., `color: var(--sy-color-fg-3)`)
  and the icon inherits.

  Sized at 18px by default — the size used in the sidebar nav. Pass `size`
  for anywhere else (typical values: 14 in dense developer rows, 16 in
  badges, 20 in headers, 24+ for ambient surfaces).
-->
<script setup lang="ts">
export type IconName =
  | "home"
  | "rooms"
  | "activity"
  | "automations"
  | "devices"
  | "settings"
  | "pages"
  | "displays"
  | "plus"
  | "search"
  | "chevron-right"
  | "chevron-down"
  | "check"
  | "close"
  | "power"
  | "bulb"
  | "thermometer"
  | "plugin"
  | "sparkle"
  | "filter"
  | "more-vertical"
  | "alert";

withDefaults(
  defineProps<{
    /** Built-in icon name. Type-checked. */
    name: IconName;
    /** Pixel size. Default 18 (the sidebar-nav size). */
    size?: number | string;
    /** Override the 1.6px default stroke. */
    strokeWidth?: number | string;
  }>(),
  { size: 18, strokeWidth: 1.6 },
);
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    :stroke-width="strokeWidth"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <template v-if="name === 'home'">
      <path d="M3 11.5 12 4l9 7.5" />
      <path d="M5.5 10v9.5h4v-6h5v6h4V10" />
    </template>
    <template v-else-if="name === 'rooms'">
      <rect x="3.5" y="4" width="7" height="7" rx="1.2" />
      <rect x="13.5" y="4" width="7" height="7" rx="1.2" />
      <rect x="3.5" y="13" width="7" height="7" rx="1.2" />
      <rect x="13.5" y="13" width="7" height="7" rx="1.2" />
    </template>
    <template v-else-if="name === 'activity'">
      <path d="M3 12h4l2-7 4 14 2-7h6" />
    </template>
    <template v-else-if="name === 'automations'">
      <circle cx="12" cy="12" r="2.4" />
      <path d="M12 4v3M12 17v3M20 12h-3M7 12H4M17.7 6.3l-2.1 2.1M8.4 15.6l-2.1 2.1M17.7 17.7l-2.1-2.1M8.4 8.4 6.3 6.3" />
    </template>
    <template v-else-if="name === 'devices'">
      <rect x="3.5" y="4.5" width="17" height="11" rx="1.5" />
      <path d="M8 19h8M12 15.5v3.5" />
    </template>
    <template v-else-if="name === 'settings'">
      <circle cx="12" cy="12" r="2.6" />
      <path d="M19.4 14.4a7.6 7.6 0 0 0 .1-1.5 7.6 7.6 0 0 0-.1-1.5l2-1.6-2-3.4-2.4.9a7.5 7.5 0 0 0-2.6-1.5l-.4-2.5h-4l-.4 2.5a7.5 7.5 0 0 0-2.6 1.5l-2.4-.9-2 3.4 2 1.6a7.6 7.6 0 0 0 0 3l-2 1.6 2 3.4 2.4-.9a7.5 7.5 0 0 0 2.6 1.5l.4 2.5h4l.4-2.5a7.5 7.5 0 0 0 2.6-1.5l2.4.9 2-3.4-2-1.6Z" />
    </template>
    <template v-else-if="name === 'pages'">
      <rect x="4.5" y="3.5" width="13" height="17" rx="1.5" />
      <path d="M7.5 8h7M7.5 12h7M7.5 16h4" />
    </template>
    <template v-else-if="name === 'displays'">
      <rect x="2.5" y="4.5" width="19" height="12" rx="1.5" />
      <path d="M8 20h8M12 16.5v3.5" />
    </template>
    <template v-else-if="name === 'plus'">
      <path d="M12 5v14M5 12h14" />
    </template>
    <template v-else-if="name === 'search'">
      <circle cx="11" cy="11" r="6" />
      <path d="m20 20-4.3-4.3" />
    </template>
    <template v-else-if="name === 'chevron-right'">
      <path d="m9 6 6 6-6 6" />
    </template>
    <template v-else-if="name === 'chevron-down'">
      <path d="m6 9 6 6 6-6" />
    </template>
    <template v-else-if="name === 'check'">
      <path d="m5 12 5 5 9-11" />
    </template>
    <template v-else-if="name === 'close'">
      <path d="M6 6l12 12M18 6 6 18" />
    </template>
    <template v-else-if="name === 'power'">
      <path d="M12 4v8" />
      <path d="M7 6.5a8 8 0 1 0 10 0" />
    </template>
    <template v-else-if="name === 'bulb'">
      <path d="M9 18h6M10 21h4" />
      <path d="M8.5 14a6 6 0 1 1 7 0 4 4 0 0 0-1.5 3v.5h-4v-.5a4 4 0 0 0-1.5-3Z" />
    </template>
    <template v-else-if="name === 'thermometer'">
      <path d="M10 14V5a2 2 0 0 1 4 0v9" />
      <circle cx="12" cy="17" r="3.2" />
    </template>
    <template v-else-if="name === 'plugin'">
      <path d="M9 2v4M15 2v4M9 18v4M15 18v4" />
      <rect x="4" y="6" width="16" height="12" rx="2" />
    </template>
    <template v-else-if="name === 'sparkle'">
      <path d="M12 3v3.5M12 17.5V21M3 12h3.5M17.5 12H21M6 6l2.5 2.5M15.5 15.5 18 18M6 18l2.5-2.5M15.5 8.5 18 6" />
    </template>
    <template v-else-if="name === 'filter'">
      <path d="M3 5h18l-7 9v6l-4-2v-4z" />
    </template>
    <template v-else-if="name === 'more-vertical'">
      <!-- Three filled dots; the only icon that breaks the stroke convention,
           because hollow rings read as zero/null instead of "more actions". -->
      <circle cx="12" cy="5"  r="1.6" fill="currentColor" stroke="none" />
      <circle cx="12" cy="12" r="1.6" fill="currentColor" stroke="none" />
      <circle cx="12" cy="19" r="1.6" fill="currentColor" stroke="none" />
    </template>
    <template v-else-if="name === 'alert'">
      <!-- Warning triangle with exclamation. Stroke triangle + line + dot. -->
      <path d="M12 4 L21 19 L3 19 Z" />
      <path d="M12 10v4" />
      <circle cx="12" cy="17" r="0.8" fill="currentColor" stroke="none" />
    </template>
  </svg>
</template>
