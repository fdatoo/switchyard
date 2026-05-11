import { useAuthStore } from "@/data/auth-store";

interface NavItem {
  id: string;
  label: string;
  path: string;
}

const PRIMARY_NAV: NavItem[] = [
  { id: "home", label: "Home", path: "/_authed/home" },
  { id: "rooms", label: "Rooms", path: "/_authed/rooms" },
  { id: "activity", label: "Activity", path: "/_authed/activity" },
  { id: "automations", label: "Automations", path: "/_authed/automations" },
  { id: "devices", label: "Devices", path: "/_authed/devices" },
  { id: "settings", label: "Settings", path: "/_authed/settings" },
];

function isActive(navPath: string, currentPath: string): boolean {
  return currentPath === navPath || currentPath.startsWith(navPath + "/");
}

interface SidebarProps {
  currentPath?: string;
}

export function Sidebar({ currentPath = typeof window !== "undefined" ? window.location.pathname : "/" }: SidebarProps) {
  const user = useAuthStore((s) => s.user);

  return (
    <nav
      aria-label="Primary navigation"
      style={{
        display: "flex",
        flexDirection: "column",
        width: "200px",
        minHeight: "100vh",
        background: "var(--sy-color-sidebar)",
        borderRight: "1px solid var(--sy-color-line)",
        padding: "14px 10px",
        fontSize: "13px",
        gap: "4px",
        flexShrink: 0,
      }}
    >
      {/* Brand */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "9px",
          padding: "4px 8px 16px",
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

      {/* Primary nav */}
      {PRIMARY_NAV.map((item) => {
        const active = isActive(item.path, currentPath);
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
            }}
          >
            <span
              aria-hidden="true"
              style={{
                width: "16px",
                height: "16px",
                borderRadius: "5px",
                background: active ? "var(--sy-color-accent)" : "var(--sy-color-fg-5)",
                flexShrink: 0,
                opacity: active ? 1 : 0.6,
              }}
            />
            {item.label}
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
        }}
        data-testid="displays-empty"
      >
        No displays yet.
      </div>

      {/* Spacer */}
      <div style={{ flex: 1 }} />

      {/* User pill */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "9px",
          padding: "8px 10px",
          borderRadius: "var(--sy-radius-sm)",
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
