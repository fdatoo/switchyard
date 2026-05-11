/**
 * Language registry — maps language ids to their resolved theme ids.
 * CSS files are imported in web/src/theme/index.css.
 */

export type Language = "friendly" | "ambient" | "developer";
export type ThemeMode = "light" | "dark" | "system";
export type ResolvedTheme = "friendly-light" | "friendly-dark" | "ambient" | "developer";

/**
 * Resolve a language + mode preference to a concrete data-theme id.
 * Ambient and developer are always dark; only friendly respects mode.
 */
export function resolveTheme(language: Language, systemPrefersDark: boolean, mode: ThemeMode): ResolvedTheme {
  if (language === "ambient") return "ambient";
  if (language === "developer") return "developer";
  // friendly
  const dark = mode === "dark" || (mode === "system" && systemPrefersDark);
  return dark ? "friendly-dark" : "friendly-light";
}
