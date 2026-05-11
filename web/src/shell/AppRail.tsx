/**
 * AppRail — thin always-visible app rail (brand mark + reserved slots).
 *
 * Plan 1: This component ships but is NOT rendered in the default Shell.
 * Plan 12 (Pkl editor) will mount it when needed.
 */
export function AppRail() {
  return (
    <aside
      aria-label="App rail"
      style={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        gap: "var(--sy-space-2)",
        width: "48px",
        background: "var(--sy-color-sidebar)",
        borderRight: "1px solid var(--sy-color-line)",
        padding: "var(--sy-space-3) 0",
      }}
    >
      {/* Brand mark */}
      <div
        aria-label="Switchyard brand mark"
        style={{
          width: "22px",
          height: "22px",
          borderRadius: "var(--sy-radius-sm)",
          background:
            "linear-gradient(135deg, var(--sy-color-accent), var(--sy-color-accent-2))",
          boxShadow: "var(--sy-shadow)",
        }}
      />
      {/* Reserved slots for Plan 12 */}
    </aside>
  );
}
