<!--
  SyAvatar — user/account avatar.

  Fallback chain: image (if `src` loads) → initials (derived from `name`) →
  the accent gradient with no glyph. The image's `onerror` flips us back to
  the initials path, so a broken URL doesn't leave us with a busted-image
  icon. The `name` is also used as the `alt` for screen readers.

  Per-user colorization (hash → hue) is out of scope for v1 because the
  product is single-tenant; if multi-account browsing lands later, we can
  swap the gradient for a per-user hash without changing the API.
-->
<script setup lang="ts">
import { computed, ref } from "vue";

type Size = "sm" | "md" | "lg" | "xl";
type Shape = "circle" | "square";

const props = withDefaults(
  defineProps<{
    /** Image URL. Falls back to initials if missing or fails to load. */
    src?: string;
    /** Used to derive initials when no image, and for the alt attribute. */
    name?: string;
    size?: Size;
    shape?: Shape;
  }>(),
  { size: "md", shape: "circle" },
);

const imageBroken = ref(false);

/* Derive up to two initials. "Fynn Datoo" → "FD"; "fynn" → "F"; "" → "". */
const initials = computed(() => {
  if (!props.name) return "";
  const parts = props.name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "";
  if (parts.length === 1) return parts[0]!.charAt(0).toUpperCase();
  return (parts[0]!.charAt(0) + parts[parts.length - 1]!.charAt(0)).toUpperCase();
});

const showImage = computed(() => Boolean(props.src) && !imageBroken.value);

const classes = computed(() => [
  "sy-avatar",
  `sy-avatar--${props.size}`,
  `sy-avatar--${props.shape}`,
]);
</script>

<template>
  <span :class="classes" :aria-label="name">
    <img
      v-if="showImage"
      :src="src"
      :alt="name ?? ''"
      class="sy-avatar__img"
      @error="imageBroken = true"
    />
    <span v-else-if="initials" class="sy-avatar__initials">{{ initials }}</span>
  </span>
</template>

<style scoped>
.sy-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  overflow: hidden;
  background: linear-gradient(135deg, var(--sy-color-accent), var(--sy-color-accent-2));
  color: var(--sy-color-bg);
  font-family: var(--sy-font-body);
  font-weight: 600;
  letter-spacing: -0.01em;
  user-select: none;
}

.sy-avatar--circle { border-radius: var(--sy-radius-pill); }
.sy-avatar--square { border-radius: var(--sy-radius); }

.sy-avatar--sm { width: 20px; height: 20px; font-size: 0.625rem; }
.sy-avatar--md { width: 28px; height: 28px; font-size: 0.75rem; }
.sy-avatar--lg { width: 40px; height: 40px; font-size: 0.9375rem; }
.sy-avatar--xl { width: 64px; height: 64px; font-size: 1.25rem; }

.sy-avatar__img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.sy-avatar__initials {
  line-height: 1;
}
</style>
