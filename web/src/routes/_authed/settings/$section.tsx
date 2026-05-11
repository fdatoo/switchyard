import { lazy, Suspense } from "react";
import { SettingsNav } from "@/pages/settings/SettingsNav";
import { useSettingsBadges } from "@/pages/settings/useSettingsBadges";

type SectionId =
  | "account"
  | "drivers"
  | "pkl-config"
  | "widget-packs"
  | "displays"
  | "theme-language"
  | "diagnostics"
  | "about";

const SECTION_MAP: Record<SectionId, React.LazyExoticComponent<() => React.ReactElement>> = {
  account: lazy(() =>
    import("@/pages/settings/sections/Account").then((m) => ({ default: m.Account })),
  ),
  drivers: lazy(() =>
    import("@/pages/settings/sections/Drivers").then((m) => ({ default: m.Drivers })),
  ),
  "pkl-config": lazy(() =>
    import("@/pages/settings/sections/PklConfig").then((m) => ({ default: m.PklConfig })),
  ),
  "widget-packs": lazy(() =>
    import("@/pages/settings/sections/WidgetPacks").then((m) => ({ default: m.WidgetPacks })),
  ),
  displays: lazy(() =>
    import("@/pages/settings/sections/Displays").then((m) => ({ default: m.Displays })),
  ),
  "theme-language": lazy(() =>
    import("@/pages/settings/sections/ThemeLanguage").then((m) => ({ default: m.ThemeLanguage })),
  ),
  diagnostics: lazy(() =>
    import("@/pages/settings/sections/Diagnostics").then((m) => ({ default: m.Diagnostics })),
  ),
  about: lazy(() =>
    import("@/pages/settings/sections/About").then((m) => ({ default: m.About })),
  ),
};

const VALID_SECTIONS = Object.keys(SECTION_MAP) as SectionId[];

function isSectionId(s: string): s is SectionId {
  return VALID_SECTIONS.includes(s as SectionId);
}

interface Props {
  section?: string;
}

export function SettingsSection({ section = "" }: Props) {
  const badges = useSettingsBadges();

  const SectionComponent = isSectionId(section) ? SECTION_MAP[section] : null;

  return (
    <div
      style={{
        display: "flex",
        flex: 1,
        minHeight: "100%",
        overflow: "hidden",
      }}
    >
      <SettingsNav badges={badges} activeSection={section} />
      <main
        style={{
          flex: 1,
          overflowY: "auto",
          padding: "var(--sy-space-6)",
        }}
      >
        {SectionComponent ? (
          <Suspense fallback={null}>
            <SectionComponent />
          </Suspense>
        ) : (
          <p
            style={{
              color: "var(--sy-color-fg-4)",
              fontStyle: "italic",
            }}
          >
            Section not found: {section || "(none)"}
          </p>
        )}
      </main>
    </div>
  );
}
