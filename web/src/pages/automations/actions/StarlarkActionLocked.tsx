import type { ActionDraft } from "../useAutomationEditor";
import { LockedFieldBanner } from "../LockedFieldBanner";

interface StarlarkActionLockedProps {
  action: ActionDraft;
}

/**
 * StarlarkActionLocked — renders a Starlark action body as a read-only locked panel.
 *
 * Shows: grey monospace preview (3 lines), "starlark" chip, "View in Pkl editor →" link.
 */
export function StarlarkActionLocked({ action }: StarlarkActionLockedProps) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--sy-space-2)" }}>
      <LockedFieldBanner
        fieldType="action"
        filePath={action.starlarkFilePath}
        line={action.starlarkLine}
      />
      <div
        style={{
          padding: "var(--sy-space-2) var(--sy-space-3)",
          background: "var(--sy-color-surface-2)",
          borderRadius: "var(--sy-radius-sm)",
          border: "1px solid var(--sy-color-line)",
          display: "flex",
          alignItems: "flex-start",
          gap: "var(--sy-space-2)",
        }}
      >
        <span
          style={{
            display: "inline-block",
            padding: "1px var(--sy-space-2)",
            borderRadius: "var(--sy-radius-pill)",
            background: "var(--sy-color-purple)",
            color: "var(--sy-color-bg)",
            fontSize: "0.6875rem",
            fontWeight: 600,
            flexShrink: 0,
          }}
        >
          starlark
        </span>
        <pre
          style={{
            margin: 0,
            flex: 1,
            fontSize: "0.8125rem",
            color: "var(--sy-color-fg-3)",
            fontFamily: "var(--sy-font-mono, monospace)",
            whiteSpace: "pre-wrap",
            overflow: "hidden",
            display: "-webkit-box",
            WebkitLineClamp: 3,
            WebkitBoxOrient: "vertical",
          }}
        >
          {action.starlarkBody ?? ""}
        </pre>
      </div>
    </div>
  );
}
