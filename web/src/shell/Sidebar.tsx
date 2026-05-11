import type { ComponentType, SVGProps } from "react";
import { useAuthStore } from "@/data/auth-store";
import { useVocab, type RouteId } from "../theme/vocab";
import {
  HomeIcon,
  RoomsIcon,
  ActivityIcon,
  AutomationsIcon,
  DevicesIcon,
  SettingsIcon,
} from "./icons";

interface NavItem {
  id: RouteId;
  path: string;
  Icon: ComponentType<SVGProps<SVGSVGElement> & { size?: number }>;
}

const PRIMARY_NAV: NavItem[] = [
  { id: "home", path: "/_authed/home", Icon: HomeIcon },
  { id: "rooms", path: "/_authed/rooms", Icon: RoomsIcon },
  { id: "activity", path: "/_authed/activity", Icon: ActivityIcon },
  { id: "automations", path: "/_authed/automations", Icon: AutomationsIcon },
  { id: "devices", path: "/_authed/devices", Icon: DevicesIcon },
  { id: "settings", path: "/_authed/settings", Icon: SettingsIcon },
];

function isActive(navPath: string, currentPath: string): boolean {
  return currentPath === navPath || currentPath.startsWith(navPath + "/");
}

interface SidebarProps {
  currentPath?: string;
}

export function Sidebar({ currentPath = typeof window !== "undefined" ? window.location.pathname : "/" }: SidebarProps) {
  const user = useAuthStore((s) => s.user);
  const vocab = useVocab();

  return (
    <nav
      aria-label="Primary navigation"
      style={{
        position: "sticky",
        top: 0,
        display: "flex",
        flexDirection: "column",
        width: "200px",
        height: "100vh",
        maxHeight: "100vh",
        background: "var(--sy-color-sidebar)",
        borderRight: "1px solid var(--sy-color-line)",
        padding: "14px 10px",
        fontSize: "13px",
        flexShrink: 0,
        overflow: "hidden",
      }}
    >
      {/* Brand */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "9px",
          padding: "4px 8px 16px",
          flexShrink: 0,
        }}
      >
        <div
          aria-hidden="true"
          style={{
            width: "22px",
            height: "22px",
            borderRadius: "7px",
            background:
              "linear-gradient(135deg, var(--sy-color-accent), var(--sy-color-accent-2))",
            boxShadow: "var(--sy-shadow)",
            flexShrink: 0,
          }}
        />
        <span
          style={{
            fontWeight: 600,
            color: "var(--sy-color-fg)",
            letterSpacing: "-0.01em",
            fontSize: "14px",
          }}
        >
          Switchyard
        </span>
      </div>

      {/* Middle region: scrollable nav + pages + displays */}
      <div
        style={{
          flex: 1,
          overflowY: "auto",
          display: "flex",
          flexDirection: "column",
          gap: "4px",
        }}
      >
        {/* Primary nav */}
        {PRIMARY_NAV.map((item, index) => {
          const active = isActive(item.path, currentPath);
          const Icon = item.Icon;
          return (
            <a
              key={item.id}
              href={item.path}
              aria-current={active ? "page" : undefined}
              data-nav-id={item.id}
              data-active={active ? "true" : "false"}
              onClick={(e) => {
                e.preventDefault();
                window.location.assign(item.path);
              }}
              style={{
                display: "flex",
                alignItems: "center",
                gap: "10px",
                padding: "7px 10px",
                borderRadius: "var(--sy-radius-sm)",
                color: active ? "var(--sy-color-fg)" : "var(--sy-color-fg-2)",
                background: active ? "var(--sy-color-surface-1)" : "transparent",
                boxShadow: active ? "var(--sy-shadow)" : "none",
                textDecoration: "none",
                cursor: "default",
                transition: "background var(--sy-motion-fast)",
                flexShrink: 0,
              }}
            >
              <span
                aria-hidden="true"
                style={{
                  display: "inline-flex",
                  color: active ? "var(--sy-color-accent)" : "var(--sy-color-fg-3)",
                  flexShrink: 0,
                }}
              >
                <Icon size={18} />
              </span>
              {vocab.label(item.id)}
              <kbd className="kbd-shortcut" aria-hidden="true">⌘{index + 1}</kbd>
            </a>
          );
        })}

        {/* Pages section */}
        <div
          style={{
            fontSize: "10.5px",
            letterSpacing: "0.1em",
            textTransform: "uppercase",
            color: "var(--sy-color-fg-4)",
            padding: "14px 10px 6px",
            flexShrink: 0,
          }}
        >
          Pages
        </div>
        <div
          style={{
            padding: "4px 10px",
            fontSize: "11.5px",
            color: "var(--sy-color-fg-5)",
            fontStyle: "italic",
            flexShrink: 0,
          }}
          data-testid="pages-empty"
        >
          No custom pages yet.
        </div>

        {/* Displays section */}
        <div
          style={{
            fontSize: "10.5px",
            letterSpacing: "0.1em",
            textTransform: "uppercase",
            color: "var(--sy-color-fg-4)",
            padding: "14px 10px 6px",
            flexShrink: 0,
          }}
        >
          Displays
        </div>
        <div
          style={{
            padding: "4px 10px",
            fontSize: "11.5px",
            color: "var(--sy-color-fg-5)",
            fontStyle: "italic",
            flexShrink: 0,
          }}
          data-testid="displays-empty"
        >
          No displays yet.
        </div>
      </div>

      {/* User pill */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "9px",
          padding: "8px 10px",
          borderRadius: "var(--sy-radius-sm)",
          flexShrink: 0,
        }}
        data-testid="user-pill"
      >
        <div
          aria-hidden="true"
          style={{
            width: "22px",
            height: "22px",
            borderRadius: "var(--sy-radius-pill)",
            background:
              "linear-gradient(135deg, var(--sy-color-accent), var(--sy-color-accent-2))",
            flexShrink: 0,
          }}
        />
        <div>
          {user ? (
            <div style={{ fontSize: "12.5px", fontWeight: 500, color: "var(--sy-color-fg)" }}>
              {user.displayName}
            </div>
          ) : (
            <a
              href="/login"
              style={{
                fontSize: "12.5px",
                fontWeight: 500,
                color: "var(--sy-color-accent)",
                textDecoration: "none",
              }}
            >
              Sign in
            </a>
          )}
        </div>
      </div>
    </nav>
  );
}
