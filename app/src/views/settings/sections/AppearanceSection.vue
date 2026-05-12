<!--
  AppearanceSection — choose language and mode.

  Both controls render inside a SySurface so they share the same visual
  weight as the rest of the settings shell. Language is a vertical list
  of selectable rows; Mode is a single row with a 3-way segmented
  selector (Light / Dark / System) on the trailing edge.

  Light/dark/system is meaningful only when the active language supports
  both modes (developer and ambient are dark-only by design); when it
  doesn't, the segmented control is disabled and we surface a caption
  explaining why.
-->
<script setup lang="ts">
import { computed } from "vue";
import {
  SyText, SyListRow, SySurface, SyBadge,
  useLanguageStore, BUILTIN_LANGUAGES,
} from "@/lib";
import type { LanguageId } from "@/lib/theme/types";
import type { ModePreference } from "@/lib/theme/language-store";

const store = useLanguageStore();

const activeLanguage = computed<LanguageId>(() => store.language);
const activeModePreference = computed<ModePreference>(() => store.modePreference);

/** True when the active language renders meaningfully different visuals
    in light vs. dark. Dark-only languages ignore the preference. */
const canChooseMode = computed<boolean>(() => {
  const desc = BUILTIN_LANGUAGES.find((l) => l.id === activeLanguage.value);
  return desc?.supportsLightDark ?? false;
});

/**
 * Which pill the segmented control should render as selected. When the
 * user *can* choose, it's their stored preference. When they can't (dark-
 * only language), we still want to communicate what's actually applied,
 * so we show the resolved mode (always "dark" in this branch) — otherwise
 * a dark-only language with preference "light" would look like nothing is
 * selected.
 */
const displayedMode = computed<ModePreference>(() => {
  if (!canChooseMode.value) return store.mode;
  return activeModePreference.value;
});

interface ModeOption {
  id: ModePreference;
  label: string;
}
const MODE_OPTIONS: readonly ModeOption[] = [
  { id: "light",  label: "Light"  },
  { id: "dark",   label: "Dark"   },
  { id: "system", label: "System" },
] as const;

function setLanguage(id: LanguageId): void { store.setLanguage(id); }
function setMode(pref: ModePreference): void { store.setModePreference(pref); }
</script>

<template>
  <section class="section">
    <header class="section__head">
      <SyText as="h1" variant="display">Appearance</SyText>
      <SyText variant="body" tone="subtle">
        How Switchyard looks. Pick the theme and, where supported, light or dark.
      </SyText>
    </header>

    <div class="section__block">
      <SyText variant="overline" tone="subtle" weight="medium">Theme</SyText>
      <SySurface padding="none">
        <SyListRow
          v-for="lang in BUILTIN_LANGUAGES"
          :key="lang.id"
          as="button"
          density="comfortable"
          interactive
          :bordered="false"
          :selected="activeLanguage === lang.id"
          @click="setLanguage(lang.id)"
        >
          <SyText weight="medium">{{ lang.label }}</SyText>
          <SyText variant="caption" tone="subtle">
            {{ lang.supportsLightDark ? "Light and dark" : "Dark only" }}
          </SyText>
          <template #trailing>
            <SyBadge v-if="activeLanguage === lang.id" intent="good" dot>
              Active
            </SyBadge>
          </template>
        </SyListRow>
      </SySurface>
    </div>

    <div class="section__block">
      <SyText variant="overline" tone="subtle" weight="medium">Mode</SyText>
      <SySurface padding="md">
        <div class="modeRow">
          <div class="modeRow__text">
            <SyText weight="medium">Appearance mode</SyText>
            <SyText variant="caption" tone="subtle">
              <template v-if="canChooseMode">
                Light, dark, or follow your system setting.
              </template>
              <template v-else>
                Light/dark isn't available for this theme; using its default mode.
              </template>
            </SyText>
          </div>
          <!--
            Segmented selector. role=radiogroup + aria-checked on each
            button gives screen-reader semantics close to a native radio.
            All three pills share one bg pill (the track) and the active
            one rides on top of it with surface-1 + shadow.
          -->
          <div
            class="segmented"
            role="radiogroup"
            aria-label="Appearance mode"
            :aria-disabled="!canChooseMode || undefined"
          >
            <button
              v-for="opt in MODE_OPTIONS"
              :key="opt.id"
              type="button"
              role="radio"
              :aria-checked="displayedMode === opt.id"
              :class="[
                'segmented__pill',
                displayedMode === opt.id && 'segmented__pill--active',
              ]"
              :disabled="!canChooseMode"
              @click="setMode(opt.id)"
            >
              {{ opt.label }}
            </button>
          </div>
        </div>
      </SySurface>
    </div>
  </section>
</template>

<style scoped>
.section {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-5);
}
.section__head {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-1);
}
.section__block {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-2);
}

.modeRow {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--sy-space-4);
}
.modeRow__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

/* The segmented track. A single inset pill with three children that share
   its rounded corners. Only the active child renders the lifted surface;
   inactive children stay transparent so the track shows through. */
.segmented {
  display: inline-flex;
  padding: 3px;
  background: var(--sy-color-surface-2);
  border-radius: 999px;
  border: 1px solid var(--sy-color-line-soft);
  flex-shrink: 0;
}
.segmented[aria-disabled="true"] {
  opacity: 0.55;
}
.segmented__pill {
  appearance: none;
  -webkit-appearance: none;
  border: 0;
  background: transparent;
  color: var(--sy-color-fg-2);
  font: inherit;
  font-weight: 500;
  font-size: 0.8125rem;
  padding: 6px 14px;
  border-radius: 999px;
  cursor: pointer;
  transition: background var(--sy-motion-fast),
              color var(--sy-motion-fast),
              box-shadow var(--sy-motion-fast);
}
.segmented__pill:hover:not(:disabled):not(.segmented__pill--active) {
  color: var(--sy-color-fg);
}
.segmented__pill:disabled {
  cursor: not-allowed;
}
.segmented__pill--active {
  background: var(--sy-color-accent);
  color: #fff;
  box-shadow: var(--sy-shadow);
}
.segmented__pill--active:hover:not(:disabled) {
  /* Slightly lighter on hover to confirm the press target; subtle so the
     surface doesn't pulse as users mouse around. */
  background: var(--sy-color-accent-2);
}
.segmented__pill:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}
</style>
