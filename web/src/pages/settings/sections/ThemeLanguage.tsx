import { useEffect, useRef, useState } from "react";
import { useLanguage } from "@/theme/language-provider";
import type { Language, ThemeMode } from "@/theme/languages/index";
import { Surface } from "@/theme/primitives/surface";

const STORAGE_KEY = "sy.theme.v2";

// Extended preferences stored alongside language and mode
interface ExtendedPrefs {
  cliPreview: boolean;
  motionReduction: "on" | "off" | "system";
}

function readExtendedPrefs(): ExtendedPrefs {
  if (typeof localStorage === "undefined") {
    return { cliPreview: false, motionReduction: "system" };
  }
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return { cliPreview: false, motionReduction: "system" };
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    return {
      cliPreview: Boolean(parsed["cliPreview"]),
      motionReduction:
        parsed["motionReduction"] === "on"
          ? "on"
          : parsed["motionReduction"] === "off"
            ? "off"
            : "system",
    };
  } catch {
    return { cliPreview: false, motionReduction: "system" };
  }
}

function saveExtendedPrefs(prefs: ExtendedPrefs): void {
  if (typeof localStorage === "undefined") return;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    const base = raw ? (JSON.parse(raw) as Record<string, unknown>) : {};
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ ...base, cliPreview: prefs.cliPreview, motionReduction: prefs.motionReduction }),
    );
  } catch {
    // ignore
  }
}

function applyMotionReduction(value: "on" | "off" | "system"): void {
  if (typeof document === "undefined") return;
  if (value === "on") {
    document.documentElement.dataset.reduceMotion = "true";
    // Inject a <style> tag that overrides all --sy-motion-* vars to 0ms
    let styleTag = document.getElementById("sy-reduce-motion") as HTMLStyleElement | null;
    if (!styleTag) {
      styleTag = document.createElement("style");
      styleTag.id = "sy-reduce-motion";
      document.head.appendChild(styleTag);
    }
    styleTag.textContent = `
      :root {
        --sy-motion-fast: 0ms !important;
        --sy-motion-medium: 0ms !important;
        --sy-motion-slow: 0ms !important;
      }
    `;
  } else {
    delete document.documentElement.dataset.reduceMotion;
    const styleTag = document.getElementById("sy-reduce-motion");
    styleTag?.remove();
  }
}

// SegmentedControl renders a group of option buttons
interface SegmentOption<T extends string> {
  key: T;
  label: string;
}

interface SegmentedControlProps<T extends string> {
  options: SegmentOption<T>[];
  value: T;
  onChange: (key: T) => void;
  label: string;
}

function SegmentedControl<T extends string>({
  options,
  value,
  onChange,
  label,
}: SegmentedControlProps<T>) {
  return (
    <div role="group" aria-label={label} style={{ display: "flex", gap: 0 }}>
      {options.map((opt, i) => {
        const isActive = opt.key === value;
        const isFirst = i === 0;
        const isLast = i === options.length - 1;
        return (
          <button
            key={opt.key}
            aria-pressed={isActive}
            onClick={() => onChange(opt.key)}
            style={{
              padding: "var(--sy-space-2) var(--sy-space-4)",
              background: isActive ? "var(--sy-color-accent)" : "var(--sy-color-surface-2)",
              color: isActive ? "var(--sy-color-bg)" : "var(--sy-color-fg)",
              border: "1px solid var(--sy-color-line)",
              borderLeft: isFirst ? "1px solid var(--sy-color-line)" : "none",
              borderRadius: isFirst
                ? "var(--sy-radius) 0 0 var(--sy-radius)"
                : isLast
                  ? "0 var(--sy-radius) var(--sy-radius) 0"
                  : "0",
              cursor: "pointer",
              font: "inherit",
              fontSize: "0.875rem",
              fontWeight: isActive ? 600 : 400,
              transition: "var(--sy-motion-fast)",
            }}
          >
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}

interface ToggleRowProps {
  label: string;
  description?: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  id: string;
}

function ToggleRow({ label, description, checked, onChange, id }: ToggleRowProps) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "flex-start",
        justifyContent: "space-between",
        gap: "var(--sy-space-4)",
        padding: "var(--sy-space-3) 0",
        borderBottom: "1px solid var(--sy-color-line)",
      }}
    >
      <label
        htmlFor={id}
        style={{ cursor: "pointer" }}
      >
        <div style={{ fontSize: "0.875rem", fontWeight: 500, color: "var(--sy-color-fg)" }}>
          {label}
        </div>
        {description && (
          <div style={{ fontSize: "0.75rem", color: "var(--sy-color-fg-4)", marginTop: "2px" }}>
            {description}
          </div>
        )}
      </label>
      <input
        id={id}
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        style={{ cursor: "pointer", width: "18px", height: "18px", flexShrink: 0 }}
      />
    </div>
  );
}

/**
 * ThemeLanguage section — four controls wired to useLanguage().
 *
 * Controls:
 *   1. Mode segmented control: Light / Dark / System → setMode()
 *   2. Language segmented control: Friendly / Ambient / Developer → setLanguage()
 *   3. CLI preview toggle: stored as sy.theme.v2.cliPreview (boolean)
 *   4. Reduce motion toggle: stored as sy.theme.v2.motionReduction
 *      When "on": sets data-reduce-motion="true" on documentElement and injects
 *      a <style> tag overriding all --sy-motion-* vars to 0ms.
 */
export function ThemeLanguage() {
  const { language, mode, setLanguage, setMode } = useLanguage();
  const [extended, setExtended] = useState<ExtendedPrefs>(() => readExtendedPrefs());
  const initialized = useRef(false);

  // Apply motion reduction on mount and when the value changes
  useEffect(() => {
    applyMotionReduction(extended.motionReduction);
  }, [extended.motionReduction]);

  // Persist extended prefs (skip first render to avoid overwriting on mount)
  useEffect(() => {
    if (!initialized.current) {
      initialized.current = true;
      return;
    }
    saveExtendedPrefs(extended);
  }, [extended]);

  const modeOptions: SegmentOption<ThemeMode>[] = [
    { key: "light", label: "Light" },
    { key: "dark", label: "Dark" },
    { key: "system", label: "System" },
  ];

  const languageOptions: SegmentOption<Language>[] = [
    { key: "friendly", label: "Friendly" },
    { key: "ambient", label: "Ambient" },
    { key: "developer", label: "Developer" },
  ];

  return (
    <div>
      <h1
        style={{
          margin: "0 0 var(--sy-space-5)",
          fontSize: "1.25rem",
          fontWeight: 600,
          color: "var(--sy-color-fg)",
        }}
      >
        Theme &amp; language
      </h1>

      <Surface
        style={{
          padding: "var(--sy-space-5)",
          border: "1px solid var(--sy-color-line)",
        }}
      >
        {/* Mode control */}
        <div style={{ marginBottom: "var(--sy-space-5)" }}>
          <p
            style={{
              margin: "0 0 var(--sy-space-3)",
              fontSize: "0.875rem",
              fontWeight: 500,
              color: "var(--sy-color-fg)",
            }}
          >
            Mode
          </p>
          <SegmentedControl
            label="Mode"
            options={modeOptions}
            value={mode}
            onChange={setMode}
          />
        </div>

        {/* Language control */}
        <div style={{ marginBottom: "var(--sy-space-5)" }}>
          <p
            style={{
              margin: "0 0 var(--sy-space-3)",
              fontSize: "0.875rem",
              fontWeight: 500,
              color: "var(--sy-color-fg)",
            }}
          >
            Language
          </p>
          <SegmentedControl
            label="Language"
            options={languageOptions}
            value={language}
            onChange={setLanguage}
          />
        </div>

        {/* Toggles */}
        <ToggleRow
          id="cli-preview"
          label="CLI preview"
          description="Show CLI-style labels instead of friendly names where applicable."
          checked={extended.cliPreview}
          onChange={(v) => setExtended((p) => ({ ...p, cliPreview: v }))}
        />
        <ToggleRow
          id="reduce-motion"
          label="Reduce motion"
          description="Override all animation tokens to 0ms for accessibility."
          checked={extended.motionReduction === "on"}
          onChange={(v) =>
            setExtended((p) => ({ ...p, motionReduction: v ? "on" : "off" }))
          }
        />
      </Surface>
    </div>
  );
}
