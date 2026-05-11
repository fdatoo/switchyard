import { Button } from "@/theme/primitives/button";
import { Surface } from "@/theme/primitives/surface";

/**
 * PklConfig section — launcher-only card.
 * The actual editor ships in Plan 12. For now renders a disabled button
 * with a Plan 12 footnote.
 */
export function PklConfig() {
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
        Pkl config
      </h1>
      <Surface
        style={{
          padding: "var(--sy-space-5)",
          border: "1px solid var(--sy-color-line)",
        }}
      >
        <h2
          style={{
            margin: "0 0 var(--sy-space-3)",
            fontSize: "1rem",
            fontWeight: 600,
            color: "var(--sy-color-fg)",
          }}
        >
          Open in Pkl editor
        </h2>
        <p
          style={{
            margin: "0 0 var(--sy-space-4)",
            fontSize: "0.875rem",
            color: "var(--sy-color-fg-3)",
            lineHeight: 1.6,
          }}
        >
          Launch the Pkl / Starlark editor to view and modify your Switchyard
          configuration files. Changes are validated before being applied to
          the daemon.
        </p>
        <Button
          variant="secondary"
          disabled
          title="Coming in Plan 12 — Pkl / Starlark Editor"
        >
          Open Pkl editor
        </Button>
        <p
          style={{
            margin: "var(--sy-space-3) 0 0",
            fontSize: "0.75rem",
            color: "var(--sy-color-fg-4)",
            fontStyle: "italic",
          }}
        >
          Coming in Plan 12 — Pkl / Starlark Editor
        </p>
      </Surface>
    </div>
  );
}
