<!--
  SyTopBar — slim chrome bar above the main content area.

  Two regions:
    - Left: breadcrumb trail (uses SyBreadcrumb).
    - Right: daemon-status dot + a search affordance that triggers the
      command palette ("Search… ⌘K"). On click the bar emits `search`;
      the consumer opens the palette.

  Designed to sit at the top of the content column (right of SySidebar)
  with a fixed height (~48px) and a hairline bottom border. Surface-1 bg
  so it reads as elevated chrome rather than part of the content.

  Status dot uses SyDot's intent + pulse vocabulary:
    - ok           → good, slow pulse
    - reconnecting → warn, fast pulse
    - down         → bad, no pulse
    - checking     → neutral, slow pulse
-->
<script setup lang="ts">
import { computed } from "vue";
import SyText from "@/lib/components/text/SyText.vue";
import SyBreadcrumb, {
  type BreadcrumbItem,
} from "@/lib/components/breadcrumb/SyBreadcrumb.vue";
import SyDot from "@/lib/components/dot/SyDot.vue";
import SyKbd from "@/lib/components/kbd/SyKbd.vue";

type DaemonStatus = "ok" | "reconnecting" | "down" | "checking";

const props = withDefaults(
  defineProps<{
    crumbs: BreadcrumbItem[];
    daemonStatus?: DaemonStatus;
    /** Placeholder text in the search trigger. */
    searchPlaceholder?: string;
  }>(),
  { daemonStatus: "ok", searchPlaceholder: "Search…" },
);

const emit = defineEmits<{
  /** Search affordance clicked — consumer opens the command palette. */
  search: [];
}>();

const dotProps = computed(() => {
  switch (props.daemonStatus) {
    case "ok":           return { intent: "good"    as const, pulse: "slow" as const, label: "Connected" };
    case "reconnecting": return { intent: "warn"    as const, pulse: "fast" as const, label: "Reconnecting" };
    case "down":         return { intent: "bad"     as const, pulse: "off"  as const, label: "Disconnected" };
    case "checking":     return { intent: "neutral" as const, pulse: "slow" as const, label: "Checking" };
  }
});
</script>

<template>
  <header class="sy-topbar">
    <div class="sy-topbar__left">
      <SyBreadcrumb :items="crumbs" />
    </div>

    <div class="sy-topbar__right">
      <span class="sy-topbar__status" :title="dotProps.label">
        <SyDot :intent="dotProps.intent" :pulse="dotProps.pulse" size="sm" :label="dotProps.label" />
      </span>
      <button type="button" class="sy-topbar__search" @click="emit('search')">
        <SyText variant="caption" tone="subtle" as="span">{{ searchPlaceholder }}</SyText>
        <SyKbd>⌘K</SyKbd>
      </button>
    </div>
  </header>
</template>

<style scoped>
.sy-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--sy-space-4);
  padding: 0 var(--sy-space-4);
  min-height: 48px;
  background: var(--sy-color-surface-1);
  border-bottom: 1px solid var(--sy-color-line);
}

.sy-topbar__left {
  display: flex;
  align-items: center;
  min-width: 0;
  flex: 1;
}

.sy-topbar__right {
  display: flex;
  align-items: center;
  gap: var(--sy-space-3);
  flex-shrink: 0;
}

.sy-topbar__status {
  display: inline-flex;
  align-items: center;
}

.sy-topbar__search {
  display: inline-flex;
  align-items: center;
  /* Placeholder hugs the left edge; the kbd chip is pushed to the right
     via `justify-content: space-between`. This matches how a real search
     input would render with a trailing affordance — Linear/Stripe pattern. */
  justify-content: space-between;
  gap: var(--sy-space-3);
  padding: 5px 6px 5px 12px;
  background: var(--sy-color-surface-2);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-pill);
  cursor: pointer;
  font: inherit;
  min-width: 220px;
  transition: background var(--sy-motion-fast), border-color var(--sy-motion-fast);
}
.sy-topbar__search:hover {
  background: var(--sy-color-surface-3);
  border-color: var(--sy-color-fg-5);
}
.sy-topbar__search:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}
</style>
