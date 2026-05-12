<!--
  SySensorValue — read-only display for numeric_sensor / binary_sensor.
-->
<script setup lang="ts">
import { SyText } from "@/lib";

defineProps<{
  /** Either a numeric value (with unit) or a boolean reading. */
  value: number | boolean;
  unit?: string;
}>();
</script>

<template>
  <div class="sy-sensor">
    <template v-if="typeof value === 'number'">
      <SyText as="span" variant="title" weight="semibold">{{ value }}</SyText>
      <SyText v-if="unit" as="span" variant="caption" tone="subtle">{{ unit }}</SyText>
    </template>
    <template v-else>
      <SyText as="span" variant="title" weight="semibold">
        {{ value ? "active" : "idle" }}
      </SyText>
    </template>
  </div>
</template>

<style scoped>
.sy-sensor { display: inline-flex; align-items: baseline; gap: var(--sy-space-1); }
</style>
