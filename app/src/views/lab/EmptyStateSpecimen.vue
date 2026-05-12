<script setup lang="ts">
import { SyText, SyEmptyState, SyIcon, SyButton, SySurface } from "@/lib";
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Empty · with CTA</SyText>
      <SySurface padding="none">
        <SyEmptyState
          title="No drivers installed"
          description="Drivers are configured in Pkl. Add one to start controlling devices."
        >
          <template #icon>
            <SyIcon name="plugin" :size="32" />
          </template>
          <template #actions>
            <SyButton intent="primary">Open Pkl config</SyButton>
            <SyButton intent="ghost">Browse driver gallery</SyButton>
          </template>
        </SyEmptyState>
      </SySurface>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Loading</SyText>
      <SySurface padding="none">
        <SyEmptyState loading title="Loading drivers…" description="Talking to the daemon." />
      </SySurface>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Error</SyText>
      <SySurface padding="none">
        <SyEmptyState
          intent="bad"
          title="Couldn't load drivers"
          description="DriverManagementService returned 500. Check the daemon logs and retry."
        >
          <template #icon>
            <SyIcon name="close" :size="32" />
          </template>
          <template #actions>
            <SyButton intent="secondary">Retry</SyButton>
          </template>
        </SyEmptyState>
      </SySurface>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Compact · inline</SyText>
      <SySurface padding="none">
        <SyEmptyState
          size="compact"
          title="No matches"
          description="Try a different search."
        >
          <template #icon>
            <SyIcon name="search" :size="18" />
          </template>
        </SyEmptyState>
      </SySurface>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
</style>
