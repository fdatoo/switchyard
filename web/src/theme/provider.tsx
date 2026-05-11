import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import type { LanguageId, Theme, ThemeMode, ThemeModePreference } from "./types";
import { developer } from "./languages/developer";

const LANGUAGES = { developer } as const;
const STORAGE_KEY = "gohome.themeMode";

type Ctx = { theme: Theme; mode: ThemeMode; modePreference: ThemeModePreference; language: LanguageId; setMode: (m: ThemeModePreference) => void; };
const ThemeContext = createContext<Ctx | null>(null);

function readPref(): ThemeModePreference {
  if (typeof localStorage === "undefined") return "system";
  const v = localStorage.getItem(STORAGE_KEY);
  return v === "light" || v === "dark" ? v : "system";
}
function resolveMode(p: ThemeModePreference): ThemeMode {
  if (p !== "system") return p;
  if (typeof matchMedia === "undefined") return "light";
  return matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [pref, setPref] = useState<ThemeModePreference>(() => readPref());
  const [, setTick] = useState(0);
  useEffect(() => {
    if (typeof matchMedia === "undefined") return;
    const mql = matchMedia("(prefers-color-scheme: dark)");
    const fn = () => setTick(n => n + 1);
    mql.addEventListener("change", fn);
    return () => mql.removeEventListener("change", fn);
  }, []);
  const mode = resolveMode(pref);
  const language: LanguageId = "developer";
  const tokens = LANGUAGES[language].modes[mode];
  useEffect(() => { document.documentElement.setAttribute("data-theme", `${language}-${mode}`); }, [language, mode]);
  const value = useMemo<Ctx>(() => ({
    theme: { language, mode, tokens }, mode, modePreference: pref, language,
    setMode: (m) => { setPref(m); if (typeof localStorage !== "undefined") localStorage.setItem(STORAGE_KEY, m); },
  }), [pref, mode, tokens]);
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTheme(): Ctx {
  const v = useContext(ThemeContext);
  if (!v) throw new Error("useTheme must be inside <ThemeProvider>");
  return v;
}
