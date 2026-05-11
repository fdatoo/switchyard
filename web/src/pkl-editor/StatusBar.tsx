// web/src/pkl-editor/StatusBar.tsx
interface StatusBarProps {
  pklVersion: string;
  unsavedCount: number;
  errorCount: number;
  formBoundCount: number;
  line: number;
  col: number;
  onFormat: () => void;
  onValidate: () => void;
  onApply: () => void;
}

export default function StatusBar({
  pklVersion,
  unsavedCount,
  errorCount,
  formBoundCount,
  line,
  col,
  onFormat,
  onValidate,
  onApply,
}: StatusBarProps) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: "var(--sy-space-3)",
        padding: "0 var(--sy-space-3)",
        height: 28,
        fontSize: 12,
        borderTop: "1px solid var(--sy-color-line)",
        background: "var(--sy-color-surface-1)",
        color: "var(--sy-color-fg-3)",
        flexShrink: 0,
      }}
    >
      <span>Pkl {pklVersion}</span>
      <span>·</span>
      <span>{unsavedCount} unsaved</span>
      <span>·</span>
      <span
        style={{
          color: errorCount > 0 ? "var(--sy-color-bad)" : undefined,
        }}
      >
        {errorCount} error{errorCount !== 1 ? "s" : ""}
      </span>
      <span>·</span>
      <span>
        {formBoundCount} form-bound region{formBoundCount !== 1 ? "s" : ""}
      </span>
      <span>·</span>
      <span>
        Ln {line}, Col {col}
      </span>
      <span>·</span>
      <span>spaces:2 · UTF-8 · LF</span>
      <div style={{ flex: 1 }} />
      <button
        onClick={onFormat}
        style={{ fontSize: 12, padding: "2px 8px", cursor: "pointer" }}
      >
        Format
      </button>
      <button
        onClick={onValidate}
        style={{ fontSize: 12, padding: "2px 8px", cursor: "pointer" }}
      >
        Validate
      </button>
      <button
        onClick={onApply}
        style={{
          fontSize: 12,
          padding: "2px 8px",
          cursor: "pointer",
          background: "var(--sy-color-accent)",
          color: "var(--sy-color-bg)",
          border: "none",
          borderRadius: "var(--sy-radius-sm)",
        }}
      >
        Apply changes
      </button>
    </div>
  );
}
