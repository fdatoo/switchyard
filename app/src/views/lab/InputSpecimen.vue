<script setup lang="ts">
import { ref } from "vue";
import { SyText, SyInput } from "@/lib";

const value = ref("");
const search = ref("");
const amount = ref("");
const broken = ref("not-an-email");
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Default · md</SyText>
      <SyInput v-model="value" placeholder="What are you naming this?" />
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">With prefix / suffix</SyText>
      <SyInput v-model="amount" placeholder="0.00">
        <template #prefix>$</template>
        <template #suffix>USD</template>
      </SyInput>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Sizes</SyText>
      <div class="col">
        <SyInput v-model="search" size="sm" placeholder="Small" />
        <SyInput v-model="search" size="md" placeholder="Medium" />
        <SyInput v-model="search" size="lg" placeholder="Large" />
      </div>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">States</SyText>
      <div class="col">
        <SyInput v-model="broken" invalid placeholder="Invalid" />
        <SyInput disabled placeholder="Disabled" />
        <SyInput readonly modelValue="readonly value" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.col { display: flex; flex-direction: column; gap: var(--sy-space-2); }
</style>
