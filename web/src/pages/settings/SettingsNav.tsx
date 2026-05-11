import type { CSSProperties } from "react";

export interface SettingsBadges {
  account?: number;
  drivers?: number;
  "widget-packs"?: number;
}

interface NavItem {
  id: string;
  label: string;
}

const NAV_ITEMS: NavItem[] = [
  { id: "account", label: "Account" },
  { id: "drivers", label: "Drivers" },
  { id: "pkl-config", label: "Pkl config" },
  { id: "widget-packs", label: "Widget packs" },
  { id: "displays", label: "Displays" },
  { id: "theme-language", label: "Theme & language" },
  { id: "diagnostics", label: "Diagnostics" },
  { id: "about", label: "About" },
];

interface SettingsNavProps {
  badges: SettingsBadges;
  /** Currently active section id (matched against NavItem.id) */
  activeSection?: string;
}

function badgeLabel(count: number): string {
  return count === 1 ? "1 alert" : `${count} alerts`;
}

export function SettingsNav({ badges, activeSection }: SettingsNavProps) {
  const activeSectionId = activeSection ?? (typeof window !== "undefined"
    ? window.location.pathname.split("/settings/")[1]?.split("/")[0]
    : undefined);

  return (
    <nav
      aria-label="Settings navigation"
      style={{
        width: "220px",
        flexShrink: 0,
        borderRight: "1px solid var(--sy-color-line)",
        padding: "var(--sy-space-4) var(--sy-space-3)",
        display: "flex",
        flexDirection: "column",
        gap: "var(--sy-space-1)",
        background: "var(--sy-color-sidebar)",
        minHeight: "100%",
      }}
    >
      <p
        style={{
          margin: "0 0 var(--sy-space-3) var(--sy-space-3)",
          fontSize: "0.6875rem",
          fontWeight: 600,
          letterSpacing: "0.08em",
          textTransform: "uppercase",
          color: "var(--sy-color-fg-4)",
        }}
      >
        Settings
      </p>
      {NAV_ITEMS.map((item) => {
        const isActive = item.id === activeSectionId;
        const badgeCount = badges[item.id as keyof SettingsBadges] ?? 0;

        const itemStyle: CSSProperties = {
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "var(--sy-space-2) var(--sy-space-3)",
          borderRadius: "var(--sy-radius)",
          fontSize: "0.8125rem",
          fontWeight: isActive ? 600 : 400,
          color: isActive ? "var(--sy-color-accent)" : "var(--sy-color-fg)",
          background: isActive ? "var(--sy-color-accent-subtle)" : "transparent",
          textDecoration: "none",
          cursor: "pointer",
          transition: "var(--sy-motion-fast)",
        };

        return (
          <a
            key={item.id}
            href={`/settings/${item.id}`}
            aria-current={isActive ? "page" : undefined}
            style={itemStyle}
          >
            <span>{item.label}</span>
            {badgeCount > 0 && (
              <span
                style={{
                  fontSize: "0.6875rem",
                  fontWeight: 500,
                  color: "var(--sy-color-warn)",
                  whiteSpace: "nowrap",
                }}
              >
                {badgeLabel(badgeCount)}
              </span>
            )}
          </a>
        );
      })}
    </nav>
  );
}
