/**
 * PklSourcePane — right-rail Pkl source viewer with dirty/locked line tinting.
 *
 * Props:
 *   - source: the regenerated Pkl source string
 *   - isDirty: true when there are unsaved edits
 *
 * Line annotation:
 *   - Lines between `// locked-region:starlark` and `// end-locked-region`
 *     get grey tint (locked region).
 *   - All other lines get orange tint when isDirty is true.
 *
 * Tabs: "Source" (default) | "Diff vs disk (N)"
 */

import { useState } from "react";

interface PklSourcePaneProps {
  source: string;
  isDirty: boolean;
}

type Tab = "source" | "diff";

interface LineInfo {
  text: string;
  locked: boolean;
  lineNumber: number;
}

function annotateLines(source: string): LineInfo[] {
  const lines = source.split("\n");
  const result: LineInfo[] = [];
  let inLocked = false;
  for (let i = 0; i < lines.length; i++) {
    const text = lines[i];
    if (text.includes("// locked-region")) {
      inLocked = true;
    }
    result.push({ text, locked: inLocked, lineNumber: i + 1 });
    if (text.includes("// end-locked-region")) {
      inLocked = false;
    }
  }
  return result;
}

export function PklSourcePane({ source, isDirty }: PklSourcePaneProps) {
  const [activeTab, setActiveTab] = useState<Tab>("source");

  const lines = annotateLines(source);
  const dirtyCount = isDirty ? lines.filter((l) => !l.locked).length : 0;

  return (
    <aside
      style={{
        width: "380px",
        flexShrink: 0,
        background: "var(--sy-color-surface-1)",
        borderRadius: "var(--sy-radius)",
        border: "1px solid var(--sy-color-line-soft)",
        overflow: "hidden",
        display: "flex",
        flexDirection: "column",
        height: "100%",
        minHeight: "200px",
      }}
      aria-label="Pkl source pane"
    >
      {/* Tab bar */}
      <div
        role="tablist"
        style={{
          display: "flex",
          borderBottom: "1px solid var(--sy-color-line)",
          flexShrink: 0,
        }}
      >
        <button
          role="tab"
          aria-selected={activeTab === "source"}
          type="button"
          onClick={() => setActiveTab("source")}
          style={{
            padding: "var(--sy-space-2) var(--sy-space-4)",
            background: "none",
            border: "none",
            borderBottom: activeTab === "source" ? "2px solid var(--sy-color-accent)" : "2px solid transparent",
            cursor: "pointer",
            fontSize: "0.8125rem",
            color: activeTab === "source" ? "var(--sy-color-fg)" : "var(--sy-color-fg-4)",
            fontWeight: activeTab === "source" ? 600 : 400,
          }}
        >
          Source
        </button>
        <button
          role="tab"
          aria-selected={activeTab === "diff"}
          type="button"
          onClick={() => setActiveTab("diff")}
          style={{
            padding: "var(--sy-space-2) var(--sy-space-4)",
            background: "none",
            border: "none",
            borderBottom: activeTab === "diff" ? "2px solid var(--sy-color-accent)" : "2px solid transparent",
            cursor: "pointer",
            fontSize: "0.8125rem",
            color: activeTab === "diff" ? "var(--sy-color-fg)" : "var(--sy-color-fg-4)",
            fontWeight: activeTab === "diff" ? 600 : 400,
          }}
        >
          Diff vs disk ({dirtyCount})
        </button>
      </div>

      {/* Content */}
      <div
        style={{
          flex: 1,
          overflow: "auto",
          fontFamily: "var(--sy-font-mono, monospace)",
          fontSize: "0.75rem",
          lineHeight: "1.5",
        }}
      >
        {activeTab === "source" ? (
          <table
            style={{
              width: "100%",
              borderCollapse: "collapse",
            }}
          >
            <tbody>
              {lines.map((line) => {
                const isLockedLine = line.locked;
                const isDirtyLine = isDirty && !line.locked;
                return (
                  <tr
                    key={line.lineNumber}
                    className={
                      isLockedLine
                        ? "pkl-source-pane__line--locked"
                        : isDirtyLine
                          ? "pkl-source-pane__line--dirty"
                          : ""
                    }
                    style={{
                      background: isLockedLine
                        ? "color-mix(in srgb, var(--sy-color-fg-5) 8%, transparent)"
                        : isDirtyLine
                          ? "color-mix(in srgb, var(--sy-color-warn) 12%, transparent)"
                          : "transparent",
                    }}
                  >
                    <td
                      style={{
                        padding: "0 var(--sy-space-2)",
                        color: "var(--sy-color-fg-4)",
                        userSelect: "none",
                        textAlign: "right",
                        minWidth: "2.5rem",
                        borderRight: "1px solid var(--sy-color-line)",
                      }}
                    >
                      {line.lineNumber}
                    </td>
                    <td
                      style={{
                        padding: "0 var(--sy-space-2)",
                        color: "var(--sy-color-fg)",
                        whiteSpace: "pre",
                      }}
                    >
                      {line.text || " "}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        ) : (
          // Diff tab — show only dirty (non-locked) lines with "+" prefix
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {isDirty ? (
                lines
                  .filter((l) => !l.locked)
                  .map((line) => (
                    <tr
                      key={line.lineNumber}
                      style={{
                        background:
                          "color-mix(in srgb, var(--sy-color-good) 10%, transparent)",
                      }}
                    >
                      <td
                        style={{
                          padding: "0 var(--sy-space-2)",
                          color: "var(--sy-color-good)",
                          userSelect: "none",
                          fontWeight: 600,
                        }}
                      >
                        +
                      </td>
                      <td
                        style={{
                          padding: "0 var(--sy-space-2)",
                          color: "var(--sy-color-fg)",
                          whiteSpace: "pre",
                        }}
                      >
                        {line.text || " "}
                      </td>
                    </tr>
                  ))
              ) : (
                <tr>
                  <td
                    colSpan={2}
                    style={{
                      padding: "var(--sy-space-3)",
                      color: "var(--sy-color-fg-4)",
                      fontStyle: "italic",
                      fontSize: "0.8125rem",
                    }}
                  >
                    No changes
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        )}
      </div>
    </aside>
  );
}
