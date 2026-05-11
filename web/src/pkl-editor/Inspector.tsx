// web/src/pkl-editor/Inspector.tsx
import type { FormBoundRegion } from "./form-bound-decorations";
import type { StarlarkRegion } from "./embedded";

interface InspectorProps {
  filePath: string;
  cursorLine: number;
  cursorCol: number;
  formBoundRegions: FormBoundRegion[];
  starlarkRegions: StarlarkRegion[];
  problems: Array<{
    line: number;
    message: string;
    severity: "error" | "warning";
  }>;
  onRevealFormEditor: (editorId: string) => void;
}

export default function Inspector({
  formBoundRegions,
  starlarkRegions,
  cursorLine,
  problems,
  onRevealFormEditor,
}: InspectorProps) {
  const activeFBR = formBoundRegions.find(
    (r) => cursorLine >= r.startLine && cursorLine <= r.endLine
  );
  const inStarlark = starlarkRegions.some(
    (r) => cursorLine >= r.startLine && cursorLine <= r.endLine
  );

  return (
    <aside
      style={{
        width: 320,
        flexShrink: 0,
        overflow: "auto",
        borderLeft: "1px solid var(--sy-color-line)",
        padding: "var(--sy-space-3)",
        fontSize: 12,
        display: "flex",
        flexDirection: "column",
        gap: "var(--sy-space-3)",
      }}
    >
      {activeFBR && (
        <section>
          <h4 style={{ margin: 0, color: "var(--sy-color-purple)" }}>
            Form-bound region
          </h4>
          <p style={{ margin: "4px 0", color: "var(--sy-color-fg-2)" }}>
            {activeFBR.label}
          </p>
          <button
            onClick={() => onRevealFormEditor(activeFBR.formEditorId)}
            style={{ fontSize: 12, cursor: "pointer" }}
          >
            Reveal in form editor →
          </button>
        </section>
      )}

      {inStarlark && (
        <section>
          <h4 style={{ margin: 0, color: "var(--sy-color-fg-2)" }}>
            Embedded Starlark
          </h4>
          <p style={{ margin: "4px 0", color: "var(--sy-color-fg-3)" }}>
            Starlark expression. Autocomplete and hover from StarlarkLs.
          </p>
        </section>
      )}

      {problems.length > 0 && (
        <section>
          <h4 style={{ margin: 0 }}>Problems — this file</h4>
          <ul style={{ margin: "4px 0", padding: 0, listStyle: "none" }}>
            {problems.map((p, i) => (
              <li
                key={i}
                style={{
                  color:
                    p.severity === "error"
                      ? "var(--sy-color-bad)"
                      : "var(--sy-color-warn)",
                  padding: "2px 0",
                }}
              >
                Ln {p.line}: {p.message}
              </li>
            ))}
          </ul>
        </section>
      )}

      {!activeFBR && !inStarlark && problems.length === 0 && (
        <p style={{ color: "var(--sy-color-fg-4)" }}>
          No information at cursor.
        </p>
      )}
    </aside>
  );
}
