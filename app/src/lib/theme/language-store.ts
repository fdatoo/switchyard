/**
 * Pinia store holding the active language and mode.
 *
 * The user picks a {@link ModePreference} — "light", "dark", or "system" —
 * which resolves to a concrete {@link ModeId} actually applied to the DOM.
 * Splitting preference from resolved mode lets us track the OS theme when
 * the user selects "system": the preference stays "system" across reloads,
 * but the applied mode flips whenever the OS does.
 *
 * Side effects, in order of importance:
 *   1. Sets `data-language` and `data-mode` on `<html>` — this is what
 *      actually activates the token override CSS. Token rules in
 *      `lib/tokens/*.css` are written as `[data-language="x"][data-mode="y"]`
 *      so they cascade from the document root to every descendant.
 *   2. Persists the language + preference to `localStorage` under
 *      {@link STORAGE_KEY} so reloads restore the user's choice.
 *   3. Enforces the `supportsLightDark` rule: dark-only languages (developer,
 *      ambient) snap to their default mode regardless of preference.
 *   4. Subscribes to `prefers-color-scheme` so the resolved mode tracks the
 *      OS when preference is "system".
 */

import { defineStore } from "pinia";
import { computed, ref, watch } from "vue";
import { BUILTIN_LANGUAGES } from "./types";
import type { LanguageDescriptor, LanguageId, ModeId } from "./types";

/** What the user *selected* — distinct from the {@link ModeId} we apply. */
export type ModePreference = "light" | "dark" | "system";

/**
 * Key under which the current theme is persisted. Bumped to `v4` because
 * the shape changed (was `mode: ModeId`, now `preference: ModePreference`).
 * Older entries still parse — see {@link loadPersisted}.
 */
const STORAGE_KEY = "sy.theme.v4";

interface PersistedTheme {
  language: LanguageId;
  preference: ModePreference;
}

function loadPersisted(): PersistedTheme {
  if (typeof window === "undefined") {
    return { language: "friendly", preference: "system" };
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<PersistedTheme & { mode: ModeId }>;
      if (parsed.language) {
        /* Forward-compat: v3 stored `mode: ModeId`. If we find that, treat
           the explicit light/dark choice as the new preference. */
        const pref: ModePreference =
          parsed.preference ?? (parsed.mode as ModePreference | undefined) ?? "system";
        return { language: parsed.language, preference: pref };
      }
    }
  } catch {
    /* Corrupt localStorage entry — fall through to default. */
  }
  return { language: "friendly", preference: "system" };
}

function descriptorFor(language: LanguageId): LanguageDescriptor | undefined {
  return BUILTIN_LANGUAGES.find((d) => d.id === language);
}

/** Reads the OS preference once; for reactive use, subscribe via matchMedia. */
function readSystemDark(): boolean {
  if (typeof window === "undefined" || !window.matchMedia) return false;
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

/**
 * Reduce a (language, preference, systemDark) triple to the {@link ModeId}
 * we actually apply. Dark-only languages always win over the user's
 * preference so the UI is never asked to render a combination it has no
 * tokens for. `systemDark` is passed in (rather than read inline) so the
 * caller controls reactivity — we drive it off a Vue ref bound to the
 * `prefers-color-scheme` media query.
 */
function resolveMode(
  language: LanguageId,
  preference: ModePreference,
  systemDark: boolean,
): ModeId {
  const desc = descriptorFor(language);
  if (desc && !desc.supportsLightDark) return desc.defaultMode;
  if (preference === "system") return systemDark ? "dark" : "light";
  return preference;
}

export const useLanguageStore = defineStore("language", () => {
  const initial = loadPersisted();
  const language = ref<LanguageId>(initial.language);
  const modePreference = ref<ModePreference>(initial.preference);
  /* Reactive mirror of `prefers-color-scheme: dark`. Updated by the
     matchMedia listener registered below. The `mode` computed reads this
     so the resolved mode tracks OS changes when preference is "system". */
  const systemDark = ref<boolean>(readSystemDark());
  const mode = computed<ModeId>(() =>
    resolveMode(language.value, modePreference.value, systemDark.value),
  );

  function setLanguage(next: LanguageId): void {
    language.value = next;
    /* Mode is computed, so it re-derives automatically. Dark-only languages
       still snap to their default regardless of preference (resolveMode
       handles it), so no manual override needed here. */
  }

  function setModePreference(next: ModePreference): void {
    modePreference.value = next;
  }

  /**
   * Legacy single-axis setter. Treated as setting the preference outright;
   * "system" can't be expressed here. Kept for callers that haven't migrated.
   */
  function setMode(next: ModeId): void {
    modePreference.value = next;
  }

  function applyToDocument(): void {
    if (typeof document === "undefined") return;
    document.documentElement.setAttribute("data-language", language.value);
    document.documentElement.setAttribute("data-mode", mode.value);
  }

  /* Apply on creation so the very first paint already has the right tokens. */
  applyToDocument();

  /* Re-apply + persist on any change to language or preference (mode is
     derived from those two, so a separate watcher would be redundant). */
  watch([language, modePreference, mode], () => {
    applyToDocument();
    if (typeof window !== "undefined") {
      window.localStorage.setItem(
        STORAGE_KEY,
        JSON.stringify({
          language: language.value,
          preference: modePreference.value,
        }),
      );
    }
  });

  /* Subscribe to OS theme changes. The media-query event updates a
     reactive ref, which feeds the `mode` computed — so the chain is
     declarative: OS flips → systemDark flips → mode recomputes → watcher
     re-applies attributes. No preference re-assignment trickery needed. */
  if (typeof window !== "undefined" && window.matchMedia) {
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = (e: MediaQueryListEvent): void => {
      systemDark.value = e.matches;
    };
    if ("addEventListener" in mq) mq.addEventListener("change", onChange);
    else (mq as MediaQueryList).addListener(onChange); // Safari < 14 fallback
  }

  return { language, mode, modePreference, setLanguage, setMode, setModePreference };
});
