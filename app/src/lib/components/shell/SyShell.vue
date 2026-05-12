<!--
  SyShell — full desktop app chrome.

  Assembles the three structural pieces into the canonical two-column layout:
    - Top: optional reconnect banner (appears when daemon-status is
      `reconnecting` or `down`).
    - Left: SySidebar (200px fixed).
    - Right: flex column with SyTopBar pinned to top and a scrollable
      `<main>` for the route's content.

  All sidebar and topbar configuration flows through props on this shell;
  emits proxy out so consumers wire one router + one search handler at the
  shell level rather than re-wiring both on every page.

  Layout pitfalls learned from C10 and patched here:
    - `height: 100vh` on the root with `min-height: 0` on flex children so
      the sidebar+main row fills the viewport and the content scrolls
      inside `<main>` rather than the body.
    - `box-sizing: border-box` (set globally in styles.css) so the 200px
      sidebar isn't 200px + padding + border.
    - The body itself stays `overflow: hidden` (set globally in styles.css)
      so opening a SySheet's scroll lock doesn't fight a body scrollbar.
-->
<script setup lang="ts">
import { computed } from "vue";
import SySidebar, {
  type SidebarNavItem,
  type SidebarLink,
  type SidebarUser,
} from "@/lib/components/sidebar/SySidebar.vue";
import SyTopBar from "@/lib/components/topbar/SyTopBar.vue";
import SyText from "@/lib/components/text/SyText.vue";
import SyIcon from "@/lib/components/icon/SyIcon.vue";
import type { BreadcrumbItem } from "@/lib/components/breadcrumb/SyBreadcrumb.vue";
import type { MenuItem } from "@/lib/components/menu/types";

type DaemonStatus = "ok" | "reconnecting" | "down" | "checking";

const props = withDefaults(
  defineProps<{
    /* Sidebar */
    brand?: string;
    primary: SidebarNavItem[];
    pages?: SidebarLink[];
    displays?: SidebarLink[];
    user?: SidebarUser;
    userMenu?: MenuItem[];
    activePath?: string;

    /* TopBar */
    crumbs: BreadcrumbItem[];
    daemonStatus?: DaemonStatus;
    searchPlaceholder?: string;
  }>(),
  {
    brand: "Switchyard",
    daemonStatus: "ok",
  },
);

const emit = defineEmits<{
  navigate: [path: string];
  search: [];
  "user-menu": [id: string];
  "sign-in": [];
}>();

/* Show the reconnect banner when the daemon isn't actually responding to
   requests. `reconnecting` is informational; `down` is more urgent. The
   topbar's status dot keeps showing in parallel — the banner is the
   "you should know" signal, the dot is the "still here" signal. */
const showBanner = computed(
  () => props.daemonStatus === "reconnecting" || props.daemonStatus === "down",
);

const bannerMessage = computed(() => {
  if (props.daemonStatus === "reconnecting") return "Reconnecting to the daemon…";
  if (props.daemonStatus === "down") return "Disconnected from the daemon. Retrying…";
  return "";
});
</script>

<template>
  <div class="sy-shell">
    <div v-if="showBanner" class="sy-shell__banner" role="status" aria-live="polite">
      <SyIcon name="alert" :size="14" />
      <SyText variant="caption" weight="medium">{{ bannerMessage }}</SyText>
    </div>

    <div class="sy-shell__body">
      <SySidebar
        :brand="brand"
        :primary="primary"
        :pages="pages"
        :displays="displays"
        :user="user"
        :user-menu="userMenu"
        :active-path="activePath"
        @navigate="emit('navigate', $event)"
        @user-menu="emit('user-menu', $event)"
        @sign-in="emit('sign-in')"
      />

      <div class="sy-shell__main">
        <SyTopBar
          :crumbs="crumbs"
          :daemon-status="daemonStatus"
          :search-placeholder="searchPlaceholder"
          @search="emit('search')"
        />
        <main class="sy-shell__content">
          <slot />
        </main>
      </div>
    </div>
  </div>
</template>

<style scoped>
.sy-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: var(--sy-color-bg);
  color: var(--sy-color-fg);
}

.sy-shell__banner {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--sy-space-2);
  flex-shrink: 0;
  padding: 6px var(--sy-space-4);
  background: var(--sy-color-warn);
  color: var(--sy-color-bg);
}

/* `min-height: 0` is the key — without it, the body flex item would refuse
   to shrink past its intrinsic content height, causing the page to scroll
   vertically rather than the content area. */
.sy-shell__body {
  display: flex;
  flex: 1;
  min-height: 0;
}

.sy-shell__main {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.sy-shell__content {
  flex: 1;
  overflow-y: auto;
}
</style>
