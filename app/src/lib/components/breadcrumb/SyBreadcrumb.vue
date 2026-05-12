<!--
  SyBreadcrumb — topbar breadcrumb trail.

  Data-driven: pass an `items` array of crumbs. Each crumb has a `label` and
  an optional `to` (href). The last crumb is treated as the current page —
  rendered as plain text in `fg` tone with no link styling, regardless of
  whether it has a `to`. Earlier crumbs are links if `to` is provided, plain
  text otherwise.

  ARIA: outer `<nav aria-label="Breadcrumb">` with an ordered list. The
  current page gets `aria-current="page"`. Chevron separators are
  `aria-hidden` so screen readers don't announce them.

  Token-driven, no variants. Truncates long trails with text-overflow
  ellipsis; consumers who want a "collapse middle crumbs into …" pattern
  can build that on top.
-->
<script setup lang="ts">
import SyIcon from "@/lib/components/icon/SyIcon.vue";

export interface BreadcrumbItem {
  label: string;
  /** Href for navigation. Omit to render as plain text. The last crumb is
      always plain text regardless of `to`. */
  to?: string;
}

defineProps<{ items: BreadcrumbItem[] }>();
</script>

<template>
  <nav aria-label="Breadcrumb" class="sy-breadcrumb">
    <ol class="sy-breadcrumb__list">
      <li
        v-for="(item, index) in items"
        :key="index"
        class="sy-breadcrumb__item"
      >
        <a
          v-if="item.to && index < items.length - 1"
          :href="item.to"
          class="sy-breadcrumb__link"
        >
          {{ item.label }}
        </a>
        <span
          v-else
          class="sy-breadcrumb__current"
          :aria-current="index === items.length - 1 ? 'page' : undefined"
        >
          {{ item.label }}
        </span>
        <span
          v-if="index < items.length - 1"
          class="sy-breadcrumb__sep"
          aria-hidden="true"
        >
          <SyIcon name="chevron-right" :size="12" />
        </span>
      </li>
    </ol>
  </nav>
</template>

<style scoped>
.sy-breadcrumb {
  display: flex;
  min-width: 0;
}

.sy-breadcrumb__list {
  display: flex;
  align-items: center;
  gap: 2px;
  margin: 0;
  padding: 0;
  list-style: none;
  min-width: 0;
}

.sy-breadcrumb__item {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  min-width: 0;
}

.sy-breadcrumb__link,
.sy-breadcrumb__current {
  font-family: var(--sy-font-body);
  font-size: 0.8125rem;
  font-weight: 500;
  letter-spacing: -0.005em;
  padding: 2px 6px;
  border-radius: var(--sy-radius-sm);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 24ch;
}

.sy-breadcrumb__link {
  color: var(--sy-color-fg-3);
  text-decoration: none;
  transition: background var(--sy-motion-fast), color var(--sy-motion-fast);
}
.sy-breadcrumb__link:hover {
  background: var(--sy-color-surface-2);
  color: var(--sy-color-fg);
}
.sy-breadcrumb__link:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}

.sy-breadcrumb__current {
  color: var(--sy-color-fg);
}

.sy-breadcrumb__sep {
  display: inline-flex;
  align-items: center;
  color: var(--sy-color-fg-5);
  flex-shrink: 0;
}
</style>
