<script setup lang="ts">
import { RouterView } from "vue-router";
import { useLanguageStore } from "@/lib/theme/language-store";

// Instantiate the store at the root so its watcher applies data-language /
// data-mode to <html> as soon as the app mounts.
useLanguageStore();
</script>

<template>
  <RouterView />
</template>
