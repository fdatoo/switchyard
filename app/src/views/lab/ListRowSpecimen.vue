<script setup lang="ts">
import { SyText, SyListRow, SyDot, SyBadge, SyIcon, SyAvatar } from "@/lib";
</script>

<template>
  <div class="specimen">
    <div class="block">
      <SyText variant="label" tone="subtle">Drivers · canonical pattern</SyText>
      <SyListRow as="a" href="#" interactive>
        <template #leading>
          <SyDot intent="good" pulse="slow" size="md" />
        </template>
        <SyText weight="medium">hue_main</SyText>
        <SyText variant="caption" tone="subtle">switchyard/hue@2.4.1</SyText>
        <template #trailing>
          <SyBadge intent="good" dot pulse="slow">Running</SyBadge>
          <SyText variant="caption" tone="subtle">12 entities</SyText>
          <SyIcon name="chevron-right" :size="14" />
        </template>
      </SyListRow>
      <SyListRow as="a" href="#" interactive>
        <template #leading>
          <SyDot intent="warn" pulse="fast" size="md" />
        </template>
        <SyText weight="medium">matter_bridge</SyText>
        <SyText variant="caption" tone="subtle">switchyard/matter@1.0.3</SyText>
        <template #trailing>
          <SyBadge intent="warn" dot pulse="fast">Reconnecting</SyBadge>
          <SyText variant="caption" tone="subtle">4 entities</SyText>
          <SyIcon name="chevron-right" :size="14" />
        </template>
      </SyListRow>
      <SyListRow>
        <template #leading>
          <SyDot intent="neutral" size="md" />
        </template>
        <SyText weight="medium">shelly_legacy</SyText>
        <SyText variant="caption" tone="subtle">switchyard/shelly@0.9.1</SyText>
        <template #trailing>
          <SyBadge intent="neutral" dot>Idle</SyBadge>
          <SyText variant="caption" tone="subtle">0 entities</SyText>
        </template>
      </SyListRow>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Densities</SyText>
      <SyListRow density="compact" interactive>
        <template #leading><SyIcon name="bulb" :size="16" /></template>
        <SyText variant="caption">compact · single-line dense lists</SyText>
        <template #trailing><SyIcon name="chevron-right" :size="14" /></template>
      </SyListRow>
      <SyListRow density="comfortable" interactive>
        <template #leading><SyIcon name="bulb" :size="18" /></template>
        <SyText weight="medium">comfortable · default 2-line rows</SyText>
        <SyText variant="caption" tone="subtle">name + secondary metadata</SyText>
        <template #trailing><SyIcon name="chevron-right" :size="14" /></template>
      </SyListRow>
      <SyListRow density="spacious" interactive>
        <template #leading><SyAvatar name="Living Room" size="lg" shape="square" /></template>
        <SyText weight="semibold" variant="subtitle">spacious · hero rooms / pages</SyText>
        <SyText variant="caption" tone="subtle">More breathing room. Two lines of context fit comfortably.</SyText>
        <template #trailing><SyIcon name="chevron-right" :size="14" /></template>
      </SyListRow>
    </div>

    <div class="block">
      <SyText variant="label" tone="subtle">Settings sub-nav</SyText>
      <SyListRow density="compact" interactive>
        <SyText variant="body">Account</SyText>
      </SyListRow>
      <SyListRow density="compact" interactive>
        <SyText variant="body">Drivers</SyText>
        <template #trailing><SyBadge intent="warn" size="sm">3 updates</SyBadge></template>
      </SyListRow>
      <SyListRow density="compact" interactive>
        <SyText variant="body">Pkl config</SyText>
      </SyListRow>
      <SyListRow density="compact" interactive>
        <SyText variant="body">Widget packs</SyText>
        <template #trailing><SyBadge intent="good" size="sm">7 installed</SyBadge></template>
      </SyListRow>
    </div>
  </div>
</template>

<style scoped>
.specimen { padding: var(--sy-space-4); display: flex; flex-direction: column; gap: var(--sy-space-4); }
.block { display: flex; flex-direction: column; gap: var(--sy-space-2); }
</style>
