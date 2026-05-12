<!--
  SyEntityToggle — on/off switch for light/switch entities. Pure
  presentational: emits change(next), parent owns optimistic state +
  callCapability orchestration.
-->
<script setup lang="ts">
import { SySwitch } from "@/lib";

defineProps<{
  on: boolean;
  busy?: boolean;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "change", next: boolean): void;
}>();
</script>

<template>
  <SySwitch
    :model-value="on"
    :disabled="disabled || busy"
    @update:model-value="(v) => emit('change', v)"
  />
</template>
