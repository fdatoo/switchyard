interface TopBarProps {
  currentPath?: string;
}

/**
 * Resolve a breadcrumb label from the current pathname.
 * Reads the last meaningful segment and capitalizes it.
 */
function pathToLabel(path: string): string {
  const segments = path.replace(/\/_authed/, "").split("/").filter(Boolean);
  if (segments.length === 0) return "Home";
  const last = segments[segments.length - 1];
  // Capitalize first letter
  return last.charAt(0).toUpperCase() + last.slice(1);
}

export function TopBar({
  currentPath = typeof window !== "undefined" ? window.location.pathname : "/",
}: TopBarProps) {
  const label = pathToLabel(currentPath);

  return (
    <header
      data-testid="topbar"
      style={{
        display: "flex",
        alignItems: "center",
        gap: "12px",
        padding: "14px 24px",
        borderBottom: "1px solid var(--sy-color-line)",
        background: "var(--sy-color-bg)",
      }}
    >
      {/* Breadcrumb */}
      <nav aria-label="Breadcrumb" data-testid="breadcrumb">
        <span
          style={{
            fontSize: "13px",
            color: "var(--sy-color-fg-3)",
          }}
        >
          <b
            style={{
              color: "var(--sy-color-fg)",
              fontWeight: 500,
            }}
          >
            {label}
          </b>
        </span>
      </nav>

      {/* Spacer */}
      <div style={{ flex: 1 }} />

      {/* Status dot — placeholder (Plan 3 will wire to interestingness) */}
      <div
        aria-label="Status indicator"
        title="Status (coming in Plan 03)"
        style={{
          width: "8px",
          height: "8px",
          borderRadius: "var(--sy-radius-pill)",
          background: "var(--sy-color-good)",
        }}
      />

      {/* ⌘K palette button — clicking focuses hidden input (Plan 5 wires the palette) */}
      <button
        aria-label="Open command palette"
        onClick={() => {
          // Plan 5 will route this to the full palette; for now, no-op
        }}
        style={{
          display: "flex",
          alignItems: "center",
          gap: "8px",
          padding: "6px 12px",
          background: "var(--sy-color-surface-1)",
          border: "1px solid var(--sy-color-line)",
          borderRadius: "var(--sy-radius-pill)",
          color: "var(--sy-color-fg-4)",
          fontSize: "12.5px",
          cursor: "pointer",
          minWidth: "160px",
          boxShadow: "var(--sy-shadow)",
        }}
      >
        <span style={{ flex: 1, textAlign: "left" }}>Search...</span>
        <kbd
          style={{
            fontFamily: "var(--sy-font-numeric)",
            fontSize: "10.5px",
            padding: "1px 5px",
            background: "var(--sy-color-surface-2)",
            borderRadius: "var(--sy-radius-sm)",
            color: "var(--sy-color-fg-4)",
            border: "1px solid var(--sy-color-line)",
          }}
        >
          ⌘K
        </kbd>
      </button>
      {/* Hidden input for future palette wiring */}
      <input type="text" aria-hidden="true" tabIndex={-1} style={{ position: "absolute", opacity: 0, pointerEvents: "none", width: 0, height: 0 }} />
    </header>
  );
}
