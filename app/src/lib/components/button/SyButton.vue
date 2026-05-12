<!--
  SyButton — variant dispatcher.

  Buttons are a "shape varies per language" primitive: a pill in friendly, a
  4px-corner chip in developer, a tall glassmorphic capsule in ambient. The
  shape difference can't be expressed through tokens alone, so the lib ships
  three sibling components (`SyButtonFriendly`, `SyButtonDeveloper`,
  `SyButtonAmbient`) and this dispatcher selects between them by reading the
  active language from the language store.

  Consumers always use `SyButton`. The variant components are internal — they
  share this file's prop contract via `./types.ts`.
-->
<script setup lang="ts">
import { computed } from "vue";
import { useLanguageStore } from "@/lib/theme/language-store";
import { resolveVariant } from "@/lib/theme/variant-registry";
import type { ButtonIntent, ButtonSize } from "./types";

defineProps<{
  intent?: ButtonIntent;
  size?: ButtonSize;
  disabled?: boolean;
  /** Maps to the native `<button type=…>` attribute. Default `button`. */
  type?: "button" | "submit" | "reset";
}>();

const store = useLanguageStore();
const VariantComponent = computed(() => resolveVariant("Button", store.language));
</script>

<template>
  <component
    :is="VariantComponent"
    :intent="intent ?? 'primary'"
    :size="size ?? 'md'"
    :disabled="disabled"
    :type="type ?? 'button'"
  >
    <slot />
  </component>
</template>
