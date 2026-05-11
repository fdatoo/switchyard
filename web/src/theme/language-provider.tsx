import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { resolveTheme, type Language, type ResolvedTheme, type ThemeMode } from "./languages/index";

const STORAGE_KEY = "sy.theme.v2";

interface StoredPrefs {
  language: Language;
  mode: ThemeMode;
}

interface LanguageContextValue {
  language: Language;
  mode: ThemeMode;
  resolvedTheme: ResolvedTheme;
  setLanguage: (l: Language) => void;
  setMode: (m: ThemeMode) => void;
}

const LanguageContext = createContext<LanguageContextValue | null>(null);

function readPrefs(): StoredPrefs {
  if (typeof localStorage === "undefined") {
    return { language: "friendly", mode: "system" };
  }
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return { language: "friendly", mode: "system" };
    const parsed = JSON.parse(raw) as Partial<StoredPrefs>;
    const language: Language =
      parsed.language === "friendly" || parsed.language === "ambient" || parsed.language === "developer"
        ? parsed.language
        : "friendly";
    const mode: ThemeMode =
      parsed.mode === "light" || parsed.mode === "dark" || parsed.mode === "system"
        ? parsed.mode
        : "system";
    return { language, mode };
  } catch {
    return { language: "friendly", mode: "system" };
  }
}

function savePrefs(prefs: StoredPrefs): void {
  if (typeof localStorage !== "undefined") {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));
  }
}

function getSystemDark(): boolean {
  if (typeof matchMedia === "undefined") return false;
  return matchMedia("(prefers-color-scheme: dark)").matches;
}

export function LanguageProvider({
  children,
  initialLanguage,
}: {
  children: ReactNode;
  initialLanguage?: Language;
}) {
  const [prefs, setPrefs] = useState<StoredPrefs>(() => {
    const stored = readPrefs();
    if (initialLanguage) return { ...stored, language: initialLanguage };
    return stored;
  });
  // Track system dark preference for re-renders when system changes
  const [systemDark, setSystemDark] = useState<boolean>(() => getSystemDark());

  // Listen to system prefers-color-scheme changes
  useEffect(() => {
    if (typeof matchMedia === "undefined") return;
    const mql = matchMedia("(prefers-color-scheme: dark)");
    const handler = (e: MediaQueryListEvent) => setSystemDark(e.matches);
    mql.addEventListener("change", handler);
    return () => mql.removeEventListener("change", handler);
  }, []);

  const resolved = useMemo<ResolvedTheme>(
    () => resolveTheme(prefs.language, systemDark, prefs.mode),
    [prefs.language, prefs.mode, systemDark],
  );

  // Apply to documentElement on every change
  useEffect(() => {
    document.documentElement.dataset.theme = resolved;
    document.documentElement.dataset.language = prefs.language;
  }, [resolved, prefs.language]);

  const setLanguage = useCallback((l: Language) => {
    setPrefs((prev) => {
      const next = { ...prev, language: l };
      savePrefs(next);
      return next;
    });
  }, []);

  const setMode = useCallback((m: ThemeMode) => {
    setPrefs((prev) => {
      const next = { ...prev, mode: m };
      savePrefs(next);
      return next;
    });
  }, []);

  const value = useMemo<LanguageContextValue>(
    () => ({
      language: prefs.language,
      mode: prefs.mode,
      resolvedTheme: resolved,
      setLanguage,
      setMode,
    }),
    [prefs.language, prefs.mode, resolved, setLanguage, setMode],
  );

  return <LanguageContext.Provider value={value}>{children}</LanguageContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useLanguage(): LanguageContextValue {
  const ctx = useContext(LanguageContext);
  if (!ctx) throw new Error("useLanguage must be used inside <LanguageProvider>");
  return ctx;
}
