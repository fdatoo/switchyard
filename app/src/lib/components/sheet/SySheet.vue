<!--
  SySheet — drawer + modal.

  One primitive covers three use cases per the vision spec's "one interaction
  primitive per problem" principle:
    - `side="right"` (desktop default) — slide-in right rail for entity /
      event / driver detail. The canonical detail-rail pattern.
    - `side="bottom"` (mobile default) — bottom sheet for the same content
      on phones. Same component, different side.
    - `side="center"` — classic centered modal for confirmations and
      short forms.

  `left` and `top` mirror right/bottom for completeness.

  Behavior:
    - Teleported to `<body>` so ancestor `overflow: hidden` and stacking
      contexts can't trap the sheet.
    - Backdrop dims the page; clicking it closes the sheet (unless
      `closeOnBackdrop="false"`).
    - Escape key closes the sheet (unless `closeOnEsc="false"`).
    - Body scroll is locked while open — we toggle `overflow: hidden` on
      `<body>` so background content doesn't scroll behind the sheet.
    - Slide animations per side use the language's spring easing token.

  Layout: optional header (with title + close button), scrollable body, and
  an optional footer for sticky action rows. The header and footer don't
  scroll with the body — they're flex-shrink: 0 around a flex: 1 body.

  ARIA: `role="dialog"` + `aria-modal="true"`. Focus management beyond what
  the browser does natively is deferred — a proper focus trap (focus moves
  into the sheet on open, returns to trigger on close, Tab cycles within
  the sheet) is a future polish. For v1, the sheet is keyboard-dismissible
  and the close button is focusable.
-->
<script setup lang="ts">
import { computed, onUnmounted, watch } from "vue";
import SyIcon from "@/lib/components/icon/SyIcon.vue";
import SyText from "@/lib/components/text/SyText.vue";

type Side = "right" | "bottom" | "left" | "top" | "center";
type Size = "sm" | "md" | "lg";

const props = withDefaults(
  defineProps<{
    modelValue: boolean;
    side?: Side;
    size?: Size;
    /** Header title. Use the `header` slot for richer content. */
    title?: string;
    /** Click on the backdrop closes the sheet. Default true. */
    closeOnBackdrop?: boolean;
    /** Escape closes the sheet. Default true. */
    closeOnEsc?: boolean;
  }>(),
  {
    side: "right",
    size: "md",
    closeOnBackdrop: true,
    closeOnEsc: true,
  },
);

const emit = defineEmits<{
  "update:modelValue": [value: boolean];
}>();

function close(): void {
  emit("update:modelValue", false);
}
function onBackdropClick(): void {
  if (props.closeOnBackdrop) close();
}
function onKey(e: KeyboardEvent): void {
  if (e.key === "Escape" && props.closeOnEsc) close();
}

/**
 * Lock body scrolling without shifting page layout OR losing scroll position.
 *
 * Naive `body { overflow: hidden }` has two pitfalls:
 *   1. Removes the viewport scrollbar, freeing ~15px of horizontal space on
 *      Windows/Linux. The page reflows rightward as the sheet opens.
 *   2. When body is the scroll container, hidden overflow clamps the page's
 *      effective scroll position to 0 — meaning the page jumps to top on
 *      open, and stays at top on close.
 *
 * The robust fix is the "fixed body" technique: capture the current
 * `scrollY`, set `body { position: fixed; top: -scrollY }` so the page
 * visually stays put, and scroll-restore on unlock. The scrollbar-width
 * padding compensation prevents the horizontal shift.
 */
let savedScrollY = 0;

function lockBodyScroll(): void {
  if (typeof document === "undefined") return;
  savedScrollY = window.scrollY;
  const sbw = window.innerWidth - document.documentElement.clientWidth;
  const body = document.body;
  body.style.position = "fixed";
  body.style.top = `-${savedScrollY}px`;
  body.style.left = "0";
  body.style.right = "0";
  if (sbw > 0) body.style.paddingRight = `${sbw}px`;
}

function unlockBodyScroll(): void {
  if (typeof document === "undefined") return;
  const body = document.body;
  /* Only restore if we actually locked; protects against double-unlock. */
  if (body.style.position !== "fixed") return;
  body.style.position = "";
  body.style.top = "";
  body.style.left = "";
  body.style.right = "";
  body.style.paddingRight = "";
  window.scrollTo(0, savedScrollY);
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      lockBodyScroll();
      document.addEventListener("keydown", onKey);
    } else {
      unlockBodyScroll();
      document.removeEventListener("keydown", onKey);
    }
  },
  /* `immediate: true` so the side-effects run when the component mounts
     with `modelValue=true` (e.g., when the sheet is hydrated from a
     deep-link URL state). Without this, the Escape listener would never
     be attached for sheets opened on first paint. */
  { immediate: true },
);

onUnmounted(() => {
  unlockBodyScroll();
  if (typeof document !== "undefined") {
    document.removeEventListener("keydown", onKey);
  }
});

const sheetClass = computed(() => [
  "sy-sheet",
  `sy-sheet--${props.side}`,
  `sy-sheet--size-${props.size}`,
]);

/* One transition name per side so the slide direction matches the side. */
const transitionName = computed(() => `sy-sheet-${props.side}`);
</script>

<template>
  <Teleport to="body">
    <Transition name="sy-sheet-fade">
      <div v-if="modelValue" class="sy-sheet-backdrop" @click="onBackdropClick" />
    </Transition>
    <Transition :name="transitionName">
      <div v-if="modelValue" :class="sheetClass" role="dialog" aria-modal="true">
        <header v-if="title || $slots.header" class="sy-sheet__header">
          <div class="sy-sheet__title">
            <slot name="header">
              <SyText variant="title">{{ title }}</SyText>
            </slot>
          </div>
          <button
            class="sy-sheet__close"
            type="button"
            aria-label="Close"
            @click="close"
          >
            <SyIcon name="close" :size="18" />
          </button>
        </header>
        <div class="sy-sheet__body">
          <slot />
        </div>
        <footer v-if="$slots.footer" class="sy-sheet__footer">
          <slot name="footer" />
        </footer>
      </div>
    </Transition>
  </Teleport>
</template>

<style>
/* Unscoped: the teleport target (body) is outside this component's scope,
   so scoped styles wouldn't reach the teleported nodes. Classes are
   namespaced (`sy-sheet*`) to keep the leak risk low. */

.sy-sheet-backdrop {
  position: fixed;
  inset: 0;
  z-index: 90;
  background: var(--sy-color-overlay);
}
.sy-sheet-fade-enter-active,
.sy-sheet-fade-leave-active {
  transition: opacity 200ms ease;
}
.sy-sheet-fade-enter-from,
.sy-sheet-fade-leave-to {
  opacity: 0;
}

.sy-sheet {
  position: fixed;
  z-index: 100;
  background: var(--sy-color-surface-1);
  border: 1px solid var(--sy-color-line);
  box-shadow: var(--sy-shadow-elevated);
  display: flex;
  flex-direction: column;
  max-height: 100vh;
  max-width: 100vw;
}

/* Right rail (desktop default) — full-height, slides from right. */
.sy-sheet--right {
  top: 0;
  right: 0;
  bottom: 0;
  border-radius: var(--sy-radius-lg) 0 0 var(--sy-radius-lg);
  border-right: 0;
}
.sy-sheet--right.sy-sheet--size-sm { width: 360px; }
.sy-sheet--right.sy-sheet--size-md { width: 480px; }
.sy-sheet--right.sy-sheet--size-lg { width: 640px; }

.sy-sheet--left {
  top: 0;
  left: 0;
  bottom: 0;
  border-radius: 0 var(--sy-radius-lg) var(--sy-radius-lg) 0;
  border-left: 0;
}
.sy-sheet--left.sy-sheet--size-sm { width: 360px; }
.sy-sheet--left.sy-sheet--size-md { width: 480px; }
.sy-sheet--left.sy-sheet--size-lg { width: 640px; }

/* Bottom (mobile default) — fixed-height fraction of viewport. */
.sy-sheet--bottom {
  left: 0;
  right: 0;
  bottom: 0;
  border-radius: var(--sy-radius-lg) var(--sy-radius-lg) 0 0;
  border-bottom: 0;
}
.sy-sheet--bottom.sy-sheet--size-sm { height: 35vh; }
.sy-sheet--bottom.sy-sheet--size-md { height: 60vh; }
.sy-sheet--bottom.sy-sheet--size-lg { height: 85vh; }

.sy-sheet--top {
  left: 0;
  right: 0;
  top: 0;
  border-radius: 0 0 var(--sy-radius-lg) var(--sy-radius-lg);
  border-top: 0;
}
.sy-sheet--top.sy-sheet--size-sm { height: 35vh; }
.sy-sheet--top.sy-sheet--size-md { height: 60vh; }
.sy-sheet--top.sy-sheet--size-lg { height: 85vh; }

/* Center — classic modal. Centered via fixed top/left + 50% translate. */
.sy-sheet--center {
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  border-radius: var(--sy-radius-lg);
}
.sy-sheet--center.sy-sheet--size-sm { width: 420px; max-height: 60vh; }
.sy-sheet--center.sy-sheet--size-md { width: 560px; max-height: 75vh; }
.sy-sheet--center.sy-sheet--size-lg { width: 800px; max-height: 85vh; }

.sy-sheet__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--sy-space-3);
  padding: var(--sy-space-4) var(--sy-space-5);
  border-bottom: 1px solid var(--sy-color-line-soft);
  flex-shrink: 0;
}
.sy-sheet__title {
  min-width: 0;
  flex: 1;
}
.sy-sheet__close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  background: transparent;
  border: 0;
  border-radius: var(--sy-radius-sm);
  color: var(--sy-color-fg-3);
  cursor: pointer;
  transition: background var(--sy-motion-fast), color var(--sy-motion-fast);
  flex-shrink: 0;
}
.sy-sheet__close:hover {
  background: var(--sy-color-surface-2);
  color: var(--sy-color-fg);
}
.sy-sheet__close:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}

.sy-sheet__body {
  flex: 1;
  overflow-y: auto;
  padding: var(--sy-space-5);
}

.sy-sheet__footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--sy-space-2);
  padding: var(--sy-space-3) var(--sy-space-5);
  border-top: 1px solid var(--sy-color-line-soft);
  flex-shrink: 0;
}

/* Slide animations: each side translates from the appropriate edge.
   Ease-out cubic — same curve as the center modal. Spring overshoot would
   briefly push the panel past its resting position into the page content,
   which reads as "snap-then-correct" rather than a clean arrival. */
.sy-sheet-right-enter-active,
.sy-sheet-right-leave-active,
.sy-sheet-left-enter-active,
.sy-sheet-left-leave-active,
.sy-sheet-bottom-enter-active,
.sy-sheet-bottom-leave-active,
.sy-sheet-top-enter-active,
.sy-sheet-top-leave-active {
  transition: transform 280ms cubic-bezier(0.16, 1, 0.3, 1), opacity 200ms ease;
}

.sy-sheet-right-enter-from,
.sy-sheet-right-leave-to { transform: translateX(100%); opacity: 0.7; }

.sy-sheet-left-enter-from,
.sy-sheet-left-leave-to { transform: translateX(-100%); opacity: 0.7; }

.sy-sheet-bottom-enter-from,
.sy-sheet-bottom-leave-to { transform: translateY(100%); opacity: 0.7; }

.sy-sheet-top-enter-from,
.sy-sheet-top-leave-to { transform: translateY(-100%); opacity: 0.7; }

/* Center modal: fade + slight scale. Note the `translate(-50%, -50%)` from
   the base position must be preserved in the from/to states, otherwise
   the modal would jump to the corner mid-transition.

   No spring easing here: a scale that overshoots past 1.0 briefly paints
   the backdrop where the modal should be, which reads as "modal got too
   big for its frame." A clean ease-out cubic settles into place without
   that artifact. The slide-in sides keep the spring because edge motion
   doesn't have the same revealing-the-backdrop problem. */
.sy-sheet-center-enter-active,
.sy-sheet-center-leave-active {
  transition: transform 220ms cubic-bezier(0.16, 1, 0.3, 1), opacity 150ms ease;
}
.sy-sheet-center-enter-from,
.sy-sheet-center-leave-to {
  transform: translate(-50%, -50%) scale(0.94);
  opacity: 0;
}

[data-language="ambient"] .sy-sheet {
  -webkit-backdrop-filter: blur(24px) saturate(140%);
  backdrop-filter: blur(24px) saturate(140%);
}
</style>
