interface LockedFieldBannerProps {
  fieldType: "action" | "condition";
  filePath?: string;
  line?: number;
}

/**
 * LockedFieldBanner — shown above a section card that has Starlark-locked content.
 *
 * Styling: left-bordered warn accent, surface-2 background.
 */
export function LockedFieldBanner({ fieldType, filePath, line }: LockedFieldBannerProps) {
  const pklEditorHref = filePath
    ? `/_authed/pkl-editor/${encodeURIComponent(filePath)}${line ? `?line=${line}` : ""}`
    : null;

  return (
    <div
      role="note"
      aria-label={`Locked ${fieldType} field`}
      style={{
        background: "var(--sy-color-surface-2)",
        borderLeft: "3px solid var(--sy-color-warn)",
        padding: "var(--sy-space-2) var(--sy-space-3)",
        borderRadius: "0 var(--sy-radius-sm) var(--sy-radius-sm) 0",
        fontSize: "0.8125rem",
        color: "var(--sy-color-fg-3)",
        display: "flex",
        alignItems: "center",
        gap: "var(--sy-space-2)",
      }}
    >
      <span>
        This {fieldType} uses a Starlark expression. Edit the expression in the Pkl editor; all
        other fields remain editable.
      </span>
      {pklEditorHref && (
        <a
          href={pklEditorHref}
          style={{
            color: "var(--sy-color-accent)",
            textDecoration: "none",
            whiteSpace: "nowrap",
          }}
        >
          View in Pkl editor →
        </a>
      )}
    </div>
  );
}
