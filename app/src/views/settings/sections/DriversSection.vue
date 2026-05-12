<!--
  DriversSection — settings entrypoint to driver management.

  Driver detail UI lives on /devices, so this section is intentionally
  a quick link rather than a duplicate list. Future config (per-driver
  enable/disable, log level, etc.) will land here.
-->
<script setup lang="ts">
import { useRouter } from "vue-router";
import { SyText, SySurface, SyButton, SyIcon } from "@/lib";

const router = useRouter();
</script>

<template>
  <section class="section">
    <header class="section__head">
      <SyText as="h1" variant="display">Drivers</SyText>
      <SyText variant="body" tone="subtle">
        Manage installed driver packs and their runtime state.
      </SyText>
    </header>

    <SySurface>
      <div class="section__row">
        <div class="section__rowText">
          <SyText weight="medium">Installed drivers</SyText>
          <SyText variant="caption" tone="subtle">
            View driver state, restart, and inspect entities on the Devices page.
          </SyText>
        </div>
        <SyButton intent="secondary" @click="router.push('/devices')">
          Open Devices
          <SyIcon name="chevron-right" :size="14" />
        </SyButton>
      </div>
    </SySurface>
  </section>
</template>

<style scoped>
.section { display: flex; flex-direction: column; gap: var(--sy-space-5); }
.section__head { display: flex; flex-direction: column; gap: var(--sy-space-1); }
.section__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--sy-space-4);
}
.section__rowText {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-1);
  min-width: 0;
}
</style>
