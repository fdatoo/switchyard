// web/src/pkl-editor/FileTree.tsx
import { useMemo } from "react";

export interface FileEntry {
  path: string; // relative to ~/.switchyard/
  dirty: boolean;
  hasError: boolean;
}

interface FileTreeProps {
  files: FileEntry[];
  activePath: string;
  onSelect: (path: string) => void;
  onSearch: () => void;
}

function groupByDir(files: FileEntry[]): Map<string, FileEntry[]> {
  const map = new Map<string, FileEntry[]>();
  for (const f of files) {
    const slash = f.path.indexOf("/");
    const dir = slash >= 0 ? f.path.slice(0, slash) : "";
    const group = map.get(dir) ?? [];
    group.push(f);
    map.set(dir, group);
  }
  return map;
}

export default function FileTree({
  files,
  activePath,
  onSelect,
  onSearch,
}: FileTreeProps) {
  const groups = useMemo(() => groupByDir(files), [files]);

  return (
    <aside
      style={{
        width: 248,
        flexShrink: 0,
        overflow: "auto",
        borderRight: "1px solid var(--sy-color-line)",
        display: "flex",
        flexDirection: "column",
      }}
    >
      {/* Search affordance — opens ⌘P palette scoped to files */}
      <div
        style={{
          padding: "var(--sy-space-2) var(--sy-space-3)",
          background: "var(--sy-color-surface-1)",
          borderBottom: "1px solid var(--sy-color-line)",
        }}
      >
        <input
          readOnly
          placeholder="Find file (⌘P)…"
          onClick={onSearch}
          style={{
            width: "100%",
            background: "transparent",
            border: "none",
            color: "var(--sy-color-fg-3)",
            fontSize: 12,
            cursor: "pointer",
          }}
        />
      </div>

      <div style={{ flex: 1, overflow: "auto" }}>
        {Array.from(groups.entries()).map(([dir, entries]) => (
          <div key={dir}>
            {dir && (
              <div
                style={{
                  padding: "var(--sy-space-1) var(--sy-space-3)",
                  fontSize: 11,
                  color: "var(--sy-color-fg-4)",
                  textTransform: "uppercase",
                  letterSpacing: "0.06em",
                }}
              >
                {dir}
              </div>
            )}
            {entries.map((f) => {
              const name = f.path.includes("/")
                ? f.path.split("/").pop()!
                : f.path;
              return (
                <button
                  key={f.path}
                  onClick={() => onSelect(f.path)}
                  aria-current={f.path === activePath ? "page" : undefined}
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "var(--sy-space-1)",
                    width: "100%",
                    padding: "var(--sy-space-1) var(--sy-space-3)",
                    textAlign: "left",
                    background:
                      f.path === activePath
                        ? "var(--sy-color-accent-soft)"
                        : "transparent",
                    border: "none",
                    cursor: "pointer",
                    color: "var(--sy-color-fg)",
                  }}
                >
                  <span style={{ flex: 1, fontSize: 13 }}>{name}</span>
                  {f.dirty && (
                    <span
                      role="status"
                      aria-label="unsaved changes"
                      style={{
                        width: 6,
                        height: 6,
                        borderRadius: "50%",
                        background: "var(--sy-color-warn)",
                        flexShrink: 0,
                      }}
                    />
                  )}
                  {f.hasError && (
                    <span
                      aria-label="has errors"
                      style={{
                        width: 6,
                        height: 6,
                        borderRadius: "50%",
                        background: "var(--sy-color-bad)",
                        flexShrink: 0,
                      }}
                    />
                  )}
                </button>
              );
            })}
          </div>
        ))}
      </div>
    </aside>
  );
}
