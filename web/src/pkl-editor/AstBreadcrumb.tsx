// web/src/pkl-editor/AstBreadcrumb.tsx
interface AstBreadcrumbProps {
  path: string[];
}

export default function AstBreadcrumb({ path }: AstBreadcrumbProps) {
  if (path.length === 0) return <nav aria-label="AST path" />;
  return (
    <nav
      aria-label="AST path"
      style={{
        display: "flex",
        alignItems: "center",
        gap: 4,
        padding: "2px var(--sy-space-3)",
        fontSize: 12,
        color: "var(--sy-color-fg-3)",
        borderBottom: "1px solid var(--sy-color-line)",
        overflow: "hidden",
        whiteSpace: "nowrap",
        flexShrink: 0,
      }}
    >
      {path.map((seg, i) => (
        <span key={i} style={{ display: "flex", alignItems: "center", gap: 4 }}>
          <span>{seg}</span>
          {i < path.length - 1 && (
            <span style={{ color: "var(--sy-color-fg-4)" }}>›</span>
          )}
        </span>
      ))}
    </nav>
  );
}
