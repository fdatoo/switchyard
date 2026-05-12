<!--
  SyEmptyState — placeholder for "no data" / "loading" / "error".

  Centered icon + title + description + actions. Three states cover most
  surfaces:
    1. Loading — spinner instead of icon, dim secondary text.
       Pass `loading` and the title becomes a status line ("Loading drivers…").
    2. Empty — icon + title + description + optional CTA actions.
       The default state; use any SyIcon in the `icon` slot.
    3. Error — `intent="bad"` tints the icon red; pair with a "Retry"
       button in the `actions` slot.

  Two size steps: `default` fills a page section, `compact` slots into an
  inline space like "no matches" inside a list.

  Note: SyEmptyState does NOT decide its own surface chrome. It renders
  inside whatever container the consumer provides — a SySurface, a list
  group, a card. This keeps it composable: an "error" empty-state can
  render inside a list-of-rows surface without nesting a surface inside a
  surface.
-->
<script setup lang="ts">
import { computed } from "vue";
import SyText from "@/lib/components/text/SyText.vue";
import SySpinner from "@/lib/components/spinner/SySpinner.vue";

type Intent = "neutral" | "warn" | "bad";
type Size = "default" | "compact";

const props = withDefaults(
  defineProps<{
    /** Status line / headline. Required. */
    title: string;
    /** Secondary supporting line. Optional. */
    description?: string;
    /** Replaces the icon slot with a SySpinner and dims text. */
    loading?: boolean;
    /** Tints the icon color. Default neutral. */
    intent?: Intent;
    /** `compact` for inline use; `default` (the default) fills a section. */
    size?: Size;
  }>(),
  { loading: false, intent: "neutral", size: "default" },
);

const classes = computed(() => [
  "sy-empty",
  `sy-empty--${props.size}`,
  `sy-empty--intent-${props.intent}`,
  props.loading && "sy-empty--loading",
]);
</script>

<template>
  <div :class="classes" role="status" :aria-live="loading ? 'polite' : 'off'">
    <span class="sy-empty__icon">
      <SySpinner v-if="loading" :size="size === 'compact' ? 18 : 28" />
      <slot v-else name="icon" />
    </span>

    <SyText :variant="size === 'compact' ? 'body' : 'subtitle'" weight="medium" class="sy-empty__title">
      {{ title }}
    </SyText>

    <SyText
      v-if="description"
      :variant="size === 'compact' ? 'caption' : 'body'"
      tone="subtle"
      class="sy-empty__desc"
    >
      {{ description }}
    </SyText>

    <div v-if="$slots.actions" class="sy-empty__actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<style scoped>
.sy-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: var(--sy-space-2);
}

.sy-empty--default {
  padding: var(--sy-space-6) var(--sy-space-5);
}
.sy-empty--default .sy-empty__desc {
  max-width: 36ch;
}

.sy-empty--compact {
  padding: var(--sy-space-4) var(--sy-space-4);
  gap: var(--sy-space-1);
}
.sy-empty--compact .sy-empty__desc {
  max-width: 28ch;
}

/* Icon color flows in via CSS `color` so children using `currentColor` (SyIcon,
   SySpinner) pick it up. Intent rules below set the color. */
.sy-empty__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-bottom: var(--sy-space-1);
}
.sy-empty--intent-neutral .sy-empty__icon { color: var(--sy-color-fg-4); }
.sy-empty--intent-warn    .sy-empty__icon { color: var(--sy-color-warn); }
.sy-empty--intent-bad     .sy-empty__icon { color: var(--sy-color-bad); }

.sy-empty--loading .sy-empty__title,
.sy-empty--loading .sy-empty__desc {
  opacity: 0.7;
}

.sy-empty__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: var(--sy-space-2);
  margin-top: var(--sy-space-3);
}
</style>
