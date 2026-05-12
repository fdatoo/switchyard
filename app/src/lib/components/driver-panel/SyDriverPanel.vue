<!--
  SyDriverPanel — driver detail.

  The expanded view of one driver, rendered in the right-rail SySheet when
  a driver row is tapped on Settings → Drivers, or as a standalone surface
  on the driver's own page. Composes tier 1+2 primitives into the
  canonical layout: name + pack header, state row, entity-type
  breakdown, recent-events stack, and an actions footer.

  Data shape (`DriverInfo`) is intentionally narrow — driver metadata as
  the daemon emits it, plus a list of `EventDef` items for the recent-
  events feed. Anything richer (logs, config diff, OTel traces) lives in
  the Pkl editor or Time-machine rather than this panel.

  Actions emit intent; the consumer wires them to RPCs:
    - `restart` → DriverManagementService/Restart
    - `configure` → opens the driver's Pkl block in the editor
    - `view-logs` → opens the logs surface

  This is friendly-language render. Developer renders a similar layout
  more densely; ambient doesn't expose driver detail (operator surface,
  read-only on mobile).
-->
<script setup lang="ts">
import { computed } from "vue";
import SyText from "@/lib/components/text/SyText.vue";
import SyBadge from "@/lib/components/badge/SyBadge.vue";
import SyButton from "@/lib/components/button/SyButton.vue";
import SyEventRow from "@/lib/components/event-row/SyEventRow.vue";
import SyEmptyState from "@/lib/components/empty-state/SyEmptyState.vue";
import SyEntityRow from "@/lib/components/entity-row/SyEntityRow.vue";
import type { IconName } from "@/lib/components/icon/SyIcon.vue";
import type { Entity } from "@/data/entities";

export type DriverState = "running" | "reconnecting" | "degraded" | "stopped" | "unknown";

export interface EntityTypeCount {
  type: string;
  count: number;
}

export interface DriverEventDef {
  id: string;
  icon: IconName;
  intent: "neutral" | "accent" | "good" | "warn" | "bad" | "info" | "automation";
  title: string;
  meta?: string;
  timestamp: string;
}

export interface DriverInfo {
  name: string;
  pack: string;
  version: string;
  state: DriverState;
  /** Pre-formatted state detail line: "Connected for 4h 12m", "Reconnecting in 8s". */
  stateDetail?: string;
  entityCount: number;
  entityTypes?: EntityTypeCount[];
}

const props = defineProps<{
  driver: DriverInfo;
  /** Recent events for the per-driver feed. Empty array shows an empty state. */
  recentEvents?: DriverEventDef[];
  /** Live entities the driver currently owns. Each renders as an SyEntityRow
      with its own controls. */
  entities?: Entity[];
  /** False when the entity stream is mid-reconnect. Surfaces a small inline
      "reconnecting…" pill in the entities section header. */
  streamConnected?: boolean;
  /** Disable the action buttons while an RPC is in flight. */
  busy?: boolean;
}>();

const emit = defineEmits<{
  restart: [];
  configure: [];
  "view-logs": [];
}>();

const stateBadge = computed<{
  intent: "good" | "warn" | "bad" | "neutral";
  pulse: "slow" | "fast" | "off";
  label: string;
}>(() => {
  switch (props.driver.state) {
    case "running":      return { intent: "good",    pulse: "slow", label: "Running" };
    case "reconnecting": return { intent: "warn",    pulse: "fast", label: "Reconnecting" };
    case "degraded":     return { intent: "warn",    pulse: "slow", label: "Degraded" };
    case "stopped":      return { intent: "bad",     pulse: "off",  label: "Stopped" };
    default:             return { intent: "neutral", pulse: "off",  label: "Unknown" };
  }
});

const events = computed(() => props.recentEvents ?? []);
const entityList = computed<Entity[]>(() => props.entities ?? []);
</script>

<template>
  <section class="sy-driver">
    <header class="sy-driver__head">
      <SyText variant="title" weight="semibold">{{ driver.name }}</SyText>
      <SyText variant="caption" tone="subtle" class="sy-driver__pack">
        {{ driver.pack }}@{{ driver.version }}
      </SyText>
    </header>

    <div class="sy-driver__state">
      <SyBadge :intent="stateBadge.intent" :pulse="stateBadge.pulse" dot>
        {{ stateBadge.label }}
      </SyBadge>
      <SyText v-if="driver.stateDetail" variant="caption" tone="subtle">
        {{ driver.stateDetail }}
      </SyText>
    </div>

    <div class="sy-driver__stats">
      <div class="kv">
        <SyText variant="label" tone="subtle">Entities</SyText>
        <SyText variant="numeric" weight="medium">{{ driver.entityCount }}</SyText>
      </div>
      <div v-if="driver.entityTypes && driver.entityTypes.length > 0" class="types">
        <SyBadge
          v-for="t in driver.entityTypes"
          :key="t.type"
          appearance="soft"
          intent="neutral"
          size="sm"
        >
          {{ t.count }} {{ t.type }}{{ t.count === 1 ? "" : "s" }}
        </SyBadge>
      </div>
    </div>

    <div class="sy-driver__section">
      <div class="sy-driver__sectionHead">
        <SyText variant="label" tone="subtle">Entities</SyText>
        <SyText
          v-if="streamConnected === false"
          variant="caption"
          tone="subtle"
        >
          reconnecting…
        </SyText>
      </div>
      <div v-if="entityList.length > 0" class="sy-driver__entityList">
        <SyEntityRow v-for="e in entityList" :key="e.id" :entity="e" />
      </div>
      <SyEmptyState
        v-else
        size="compact"
        title="No entities yet"
        description="This driver hasn't registered any entities."
      />
    </div>

    <div class="sy-driver__section">
      <SyText variant="label" tone="subtle">Recent events</SyText>
      <div v-if="events.length > 0" class="events">
        <SyEventRow
          v-for="e in events"
          :key="e.id"
          :icon="e.icon"
          :intent="e.intent"
          :title="e.title"
          :meta="e.meta"
          :timestamp="e.timestamp"
        />
      </div>
      <SyEmptyState
        v-else
        size="compact"
        title="No recent events"
        description="Events from this driver will appear here."
      />
    </div>

    <footer class="sy-driver__actions">
      <SyButton intent="primary" :disabled="busy" @click="emit('restart')">
        Restart
      </SyButton>
      <SyButton intent="secondary" :disabled="busy" @click="emit('configure')">
        Configure
      </SyButton>
      <SyButton intent="ghost" :disabled="busy" @click="emit('view-logs')">
        View logs
      </SyButton>
    </footer>
  </section>
</template>

<style scoped>
.sy-driver {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-4);
}

.sy-driver__head {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.sy-driver__pack {
  font-family: var(--sy-font-numeric);
  font-feature-settings: var(--sy-numeric-feature);
}

.sy-driver__state {
  display: flex;
  align-items: center;
  gap: var(--sy-space-3);
}

.sy-driver__stats {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-2);
  padding: var(--sy-space-3) 0;
  border-top: 1px solid var(--sy-color-line-soft);
  border-bottom: 1px solid var(--sy-color-line-soft);
}
.kv {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--sy-space-3);
}
.types {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sy-space-1);
}

.sy-driver__section {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-2);
}
.sy-driver__sectionHead {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.sy-driver__entityList { display: flex; flex-direction: column; }
.events {
  /* Reset the inherited padding on .sy-event so they sit edge-to-edge in
     this panel rather than being indented like inside a SySurface card. */
  display: flex;
  flex-direction: column;
}

.sy-driver__actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sy-space-2);
  padding-top: var(--sy-space-3);
  border-top: 1px solid var(--sy-color-line-soft);
}
</style>
