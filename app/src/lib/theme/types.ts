/**
 * Public type surface for the language and theme system.
 *
 * The component library lets a user choose a {@link LanguageId} (a coordinated
 * bundle of tokens + variant components + vocabulary) and a {@link ModeId}
 * (light or dark, where the language supports it). The two axes are applied
 * as `data-language` and `data-mode` attributes on `<html>`; CSS in
 * `lib/tokens/` keys off both attributes to deliver the right token values.
 *
 * See `lib/README.md` for a longer walkthrough.
 */

import type { Component } from "vue";

/**
 * The id of a language preset. The three built-in languages have stable ids;
 * the trailing `(string & {})` keeps the union open so user-defined language
 * packs (future) can register additional ids without changing this file.
 */
export type LanguageId = "friendly" | "developer" | "ambient" | (string & {});

/** Light or dark. Only languages whose `supportsLightDark` is true honor this. */
export type ModeId = "light" | "dark";

/**
 * Metadata describing a language. Built-ins are listed in
 * {@link BUILTIN_LANGUAGES}; user-defined languages will eventually register
 * their own descriptor.
 */
export interface LanguageDescriptor {
  /** Stable id, also used as the `[data-language]` attribute value. */
  id: LanguageId;
  /** Human-readable label for UI affordances (theme switcher, settings). */
  label: string;
  /**
   * Whether this language renders meaningfully different visuals in light
   * vs. dark mode. `developer` and `ambient` are dark-only by design.
   */
  supportsLightDark: boolean;
  /** Mode to use when `supportsLightDark` is false, or as initial value. */
  defaultMode: ModeId;
}

/** The built-in languages that ship with the lib. */
export const BUILTIN_LANGUAGES: readonly LanguageDescriptor[] = [
  { id: "friendly", label: "Friendly", supportsLightDark: true, defaultMode: "light" },
  { id: "developer", label: "Developer", supportsLightDark: false, defaultMode: "dark" },
  { id: "ambient", label: "Ambient", supportsLightDark: false, defaultMode: "dark" },
] as const;

/**
 * The set of primitive slots that have language-specific component variants.
 *
 * When a primitive's per-language shape can't be expressed through tokens
 * alone (e.g., a button that's a pill in friendly, sharp chip in developer,
 * glass capsule in ambient), its name lives here and each language registers
 * a separate component implementation in {@link installBuiltinVariants}.
 *
 * Primitives that only differ by token values (color, radius, spacing) do
 * NOT need a slot — they share one component across all languages.
 */
export type SlotName = "Button" | "Input";

/** Shape of a single registration entry. Useful for batch APIs. */
export interface VariantRegistration {
  slot: SlotName;
  language: LanguageId;
  component: Component;
}
