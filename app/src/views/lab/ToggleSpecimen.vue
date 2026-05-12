<script setup lang="ts">
import { ref } from "vue";
import { SyText, SyCheckbox, SySwitch } from "@/lib";

const c1 = ref(true);
const c2 = ref(false);
const c3 = ref(true);
const cs = ref(true);
const s1 = ref(true);
const s2 = ref(false);
const ss = ref(true);
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Checkbox</SyText>
      <SyCheckbox v-model="c1">Notify me when offline</SyCheckbox>
      <SyCheckbox v-model="c2">Subscribe to weekly digests</SyCheckbox>
      <SyCheckbox indeterminate>Mixed (parent of multi-select)</SyCheckbox>
      <SyCheckbox v-model="c3" disabled>Disabled (checked)</SyCheckbox>
      <SyCheckbox invalid>Invalid (red border)</SyCheckbox>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Checkbox sizes</SyText>
      <div class="row">
        <SyCheckbox v-model="cs" size="sm">sm</SyCheckbox>
        <SyCheckbox v-model="cs" size="md">md</SyCheckbox>
        <SyCheckbox v-model="cs" size="lg">lg</SyCheckbox>
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Switch</SyText>
      <SySwitch v-model="s1">Enable notifications</SySwitch>
      <SySwitch v-model="s2">Send anonymous diagnostics</SySwitch>
      <SySwitch disabled>Disabled</SySwitch>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Switch sizes</SyText>
      <div class="row">
        <SySwitch v-model="ss" size="sm">sm</SySwitch>
        <SySwitch v-model="ss" size="md">md</SySwitch>
        <SySwitch v-model="ss" size="lg">lg</SySwitch>
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.row { display: flex; flex-wrap: wrap; gap: var(--sy-space-3); align-items: center; }
</style>
