<script setup lang="ts">
import { RouterLink } from "vue-router";
import { SyText, SyButton, BUILTIN_LANGUAGES, useLanguageStore } from "@/lib";
import type { LanguageId, ModeId } from "@/lib";
import LabSection from "./lab/LabSection.vue";
import TokenSwatches from "./lab/TokenSwatches.vue";
import TextSpecimen from "./lab/TextSpecimen.vue";
import ButtonSpecimen from "./lab/ButtonSpecimen.vue";
import SurfaceSpecimen from "./lab/SurfaceSpecimen.vue";
import BadgeSpecimen from "./lab/BadgeSpecimen.vue";
import InputSpecimen from "./lab/InputSpecimen.vue";
import ToggleSpecimen from "./lab/ToggleSpecimen.vue";
import IconSpecimen from "./lab/IconSpecimen.vue";
import KbdSpecimen from "./lab/KbdSpecimen.vue";
import DotSpecimen from "./lab/DotSpecimen.vue";
import AvatarSpecimen from "./lab/AvatarSpecimen.vue";
import SpinnerSpecimen from "./lab/SpinnerSpecimen.vue";
import ListRowSpecimen from "./lab/ListRowSpecimen.vue";
import EmptyStateSpecimen from "./lab/EmptyStateSpecimen.vue";
import TabsSpecimen from "./lab/TabsSpecimen.vue";
import NavItemSpecimen from "./lab/NavItemSpecimen.vue";
import BreadcrumbSpecimen from "./lab/BreadcrumbSpecimen.vue";
import TooltipSpecimen from "./lab/TooltipSpecimen.vue";
import SheetSpecimen from "./lab/SheetSpecimen.vue";
import MenuSpecimen from "./lab/MenuSpecimen.vue";
import DataTableSpecimen from "./lab/DataTableSpecimen.vue";
import StatusBarSpecimen from "./lab/StatusBarSpecimen.vue";
import EventRowSpecimen from "./lab/EventRowSpecimen.vue";
import StoryRowSpecimen from "./lab/StoryRowSpecimen.vue";
import AutomationCardSpecimen from "./lab/AutomationCardSpecimen.vue";
import RoomTileSpecimen from "./lab/RoomTileSpecimen.vue";
import AmbientRoomTileSpecimen from "./lab/AmbientRoomTileSpecimen.vue";
import DriverPanelSpecimen from "./lab/DriverPanelSpecimen.vue";
import SidebarSpecimen from "./lab/SidebarSpecimen.vue";
import TopBarSpecimen from "./lab/TopBarSpecimen.vue";
import ShellSpecimen from "./lab/ShellSpecimen.vue";

interface Combo {
  id: string;
  language: LanguageId;
  mode: ModeId;
  label: string;
}

const COMBOS: Combo[] = [
  { id: "friendly-light", language: "friendly", mode: "light", label: "Friendly · Light" },
  { id: "friendly-dark",  language: "friendly", mode: "dark",  label: "Friendly · Dark"  },
  { id: "developer",      language: "developer", mode: "dark", label: "Developer"        },
  { id: "ambient",        language: "ambient",   mode: "dark", label: "Ambient"          },
];

const store = useLanguageStore();
</script>

<template>
  <div class="lab">
    <header class="lab__header">
      <div class="lab__title">
        <SyText as="h1" variant="title">Component Lab</SyText>
        <SyText variant="caption" tone="subtle">
          Every primitive, in every language × mode. Edits here propagate to every consumer.
        </SyText>
      </div>
      <div class="lab__controls">
        <SyText variant="label" tone="subtle">Global theme</SyText>
        <div class="lab__chiprow">
          <button
            v-for="lang in BUILTIN_LANGUAGES"
            :key="lang.id"
            type="button"
            class="lab__chip"
            :data-active="store.language === lang.id"
            @click="store.setLanguage(lang.id)"
          >
            {{ lang.label }}
          </button>
        </div>
        <div class="lab__chiprow">
          <button
            type="button"
            class="lab__chip"
            :data-active="store.mode === 'light'"
            :disabled="!BUILTIN_LANGUAGES.find((l) => l.id === store.language)?.supportsLightDark"
            @click="store.setMode('light')"
          >
            Light
          </button>
          <button
            type="button"
            class="lab__chip"
            :data-active="store.mode === 'dark'"
            :disabled="!BUILTIN_LANGUAGES.find((l) => l.id === store.language)?.supportsLightDark"
            @click="store.setMode('dark')"
          >
            Dark
          </button>
        </div>
        <RouterLink to="/" class="lab__home">← Home</RouterLink>
      </div>
    </header>

    <LabSection title="Tokens" caption="Every semantic --sy-* token, rendered as a swatch.">
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <TokenSwatches />
        </div>
      </div>
    </LabSection>

    <LabSection title="Text" caption="Variants × tones × weights. Same component, different tokens per language.">
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <TextSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Surface"
      caption="Card/panel primitive. Token-driven: radius and shadow change per language; ambient surfaces gain backdrop-blur."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <SurfaceSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Badge"
      caption="Status pill. Three appearances (soft / solid / outline) × seven intents. color-mix() tints the background and border off the intent color."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <BadgeSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Input"
      caption="Variant components: rounded-lg with accent ring (friendly), sharp with outline (developer), large glass capsule (ambient). v-model, prefix/suffix slots, invalid/disabled/readonly states."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <InputSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Checkbox &amp; Switch"
      caption="Native input under styled overlay. Checkbox for discrete form selections; switch for instantly-applied settings. Token-driven, no variants."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <ToggleSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Icon"
      caption="One SFC keyed by name. 24×24 viewBox, 1.6px stroke, currentColor — color flows from the parent's CSS color, sizes flow from the size prop."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <IconSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Kbd"
      caption="Shortcut chip. Renders an HTML &lt;kbd&gt; so screen readers announce it as a key. Token-driven."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <KbdSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Dot"
      caption="Status dot primitive. Same intent + pulse vocabulary as the badge-internal dot, usable standalone for sidebar lights, list-row leading marks, breadcrumb status."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <DotSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Avatar"
      caption="Image → initials → gradient fallback chain. Token-driven. Broken `src` flips to the initials path automatically."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <AvatarSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Spinner"
      caption="Indeterminate loading. SVG arc rotating via CSS keyframes. Stroke proportional to size. Pulses opacity under prefers-reduced-motion."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <SpinnerSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="List Row · composite"
      caption="The canonical row layout: leading slot · main (stacked) · trailing slot. Drives every entity list and settings sub-nav. Token-driven, three density steps, interactive with hover + scale-down."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <ListRowSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Empty State · composite"
      caption="Loading / empty / error placeholder. Centered icon + title + description + actions. Renders inside whatever surface the consumer provides — no nested chrome."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <EmptyStateSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Tabs · composite"
      caption="Pill segmented control. Data-driven (pass a `tabs` array), uncontrolled-by-default. Panels are the consumer's responsibility — render them conditionally on the active id."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <TabsSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Nav Item · composite"
      caption="The sidebar nav row. Composes SyIcon + label + optional badge + optional shortcut chip (visible in developer language only). Active state lifts with surface-1 + shadow + accent icon."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <NavItemSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Breadcrumb · composite"
      caption="Topbar location trail. Data-driven (items array). Last crumb is always plain text + `aria-current=page`; earlier crumbs are links if `to` is set."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <BreadcrumbSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Tooltip · composite"
      caption="Hover/focus popover, teleported to body so ancestor `overflow:hidden` can't clip it. Four-side positioning with viewport-flip. Inverse fg/bg for the canonical popover look."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <TooltipSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Sheet · composite"
      caption="Drawer + modal. Five sides (right=desktop detail rail, bottom=mobile detail, center=modal, left/top mirrors). Teleported to body, scroll-locks, Esc to close. Sheets render at viewport scale — switch global theme to see per-language treatment."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <SheetSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Menu · composite"
      caption="Dropdown menu. Data-driven items (action / separator / header). Click a trigger to open; click outside or Esc to close. Below-trigger placement with flip-above on viewport overflow."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <MenuSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Data Table · composite"
      caption="Sortable columnar list. Data-driven (columns + rows), controlled sort via v-model:sort, named slots per column for rich cells. Developer language's primary list affordance; usable in friendly/ambient too."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <DataTableSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Status Bar · composite"
      caption="Slim bottom bar. Three regions (left / center / right) composed via slots. Used in the Pkl editor, developer-language pages, and mobile info bars."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <StatusBarSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Event Row · domain"
      caption="Single Activity feed row. Tinted icon · title (+ optional cause chain) · meta · timestamp. Used in the Activity page's All-events tab and the detail right-rail's recent-events list."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <EventRowSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Story Row · domain"
      caption="Coalesced Activity story. Larger tile + title + count + time range. Expandable to reveal constituent SyEventRows in a thread-rail. Used in the Activity page's Stories tab."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <StoryRowSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Automation Card · domain"
      caption="One automation row with trigger summary, next-run hint, Run-now, and an enable toggle. Running state shows a pulsing badge + spinner."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <AutomationCardSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Room Tile · domain"
      caption="Friendly-language room card. Avatar + name + meta header, stats body (consumer-composed badges), optional scenes footer. Whole tile is one interactive surface with the cursor-tracking hover treatment."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <RoomTileSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Ambient Room Tile · domain"
      caption="The big-glass room tile for wall displays. Per-tile fidelity (width, metric, scenes) per the spec. Oversized typography + touch targets, glassmorphic surface, alert state for fire/leak/intrusion."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <AmbientRoomTileSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Driver Panel · domain"
      caption="Driver detail card: name + pack@version, state with pulse, entity-type breakdown, recent events, action footer. Used in the Settings → Drivers right-rail SySheet."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <DriverPanelSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Sidebar · domain"
      caption="Primary navigation rail: brand · scrollable nav + sections (Pages, Displays) · user pill pinned to bottom. Data-driven; emits navigate intent."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <SidebarSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Top Bar · domain"
      caption="Slim chrome above the main content area. Breadcrumb on the left, daemon-status dot + command-palette trigger on the right."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <TopBarSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Shell · domain"
      caption="The full desktop app chrome. Reconnect banner · sidebar · topbar · scrollable content. Assembles the rest of tier 3 into one component the consumer wraps their router around."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <ShellSpecimen />
        </div>
      </div>
    </LabSection>

    <LabSection
      title="Button"
      caption="Variant components: pill in friendly, sharp chip in developer, glass capsule in ambient. The shape changes, not just the color."
    >
      <div class="lab__grid">
        <div
          v-for="c in COMBOS"
          :key="c.id"
          class="lab__cell"
          :data-language="c.language"
          :data-mode="c.mode"
        >
          <div class="lab__cellHead">
            <SyText variant="label" tone="subtle">{{ c.label }}</SyText>
          </div>
          <ButtonSpecimen />
        </div>
      </div>
    </LabSection>
  </div>
</template>

<style scoped>
.lab {
  min-height: 100vh;
  background: var(--sy-color-bg);
  color: var(--sy-color-fg);
}

.lab__header {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--sy-space-5);
  padding: var(--sy-space-5) var(--sy-space-6);
  background: var(--sy-color-bg);
  box-shadow: 0 1px 0 var(--sy-color-line);
}

.lab__title {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-1);
}

.lab__controls {
  display: flex;
  align-items: center;
  gap: var(--sy-space-3);
  flex-wrap: wrap;
}

.lab__chiprow {
  display: inline-flex;
  gap: 2px;
  padding: 2px;
  background: var(--sy-color-surface-2);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-pill);
}

.lab__chip {
  padding: 5px 14px;
  font-size: 0.8125rem;
  font-weight: 500;
  font-family: var(--sy-font-body);
  color: var(--sy-color-fg-3);
  background: transparent;
  border: 0;
  border-radius: var(--sy-radius-pill);
  cursor: pointer;
  transition: background var(--sy-motion-fast), color var(--sy-motion-fast);
}
.lab__chip:hover:not(:disabled) {
  color: var(--sy-color-fg);
}
.lab__chip[data-active="true"] {
  background: var(--sy-color-surface-1);
  color: var(--sy-color-fg);
  box-shadow: var(--sy-shadow);
}
.lab__chip:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.lab__home {
  font-size: 0.8125rem;
  color: var(--sy-color-fg-3);
  text-decoration: none;
}
.lab__home:hover {
  color: var(--sy-color-fg);
}

.lab__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: var(--sy-space-4);
}

@media (max-width: 1200px) {
  .lab__grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 720px) {
  .lab__grid { grid-template-columns: 1fr; }
}

.lab__cell {
  border-radius: var(--sy-radius-lg);
  border: 1px solid var(--sy-color-line);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.lab__cellHead {
  padding: var(--sy-space-3) var(--sy-space-4);
  border-bottom: 1px solid var(--sy-color-line-soft);
  background: var(--sy-color-surface-1);
}
</style>
