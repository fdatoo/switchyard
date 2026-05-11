import type { ActionDraft } from "../useAutomationEditor";

interface WaitActionProps {
  action: ActionDraft;
  onChange: (action: ActionDraft) => void;
}

export function WaitAction({ action, onChange }: WaitActionProps) {
  return (
    <div style={{ display: "flex", gap: "var(--sy-space-2)", alignItems: "flex-end" }}>
      <label
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "var(--sy-space-1)",
          fontSize: "0.8125rem",
          color: "var(--sy-color-fg-3)",
        }}
      >
        Duration
        <input
          type="number"
          min={0}
          value={action.durationValue ?? 0}
          onChange={(e) => onChange({ ...action, durationValue: Number(e.target.value) })}
          style={{
            padding: "var(--sy-space-1) var(--sy-space-2)",
            borderRadius: "var(--sy-radius-sm)",
            border: "1px solid var(--sy-color-line)",
            background: "var(--sy-color-surface-1)",
            color: "var(--sy-color-fg)",
            fontSize: "0.875rem",
            width: "6rem",
          }}
          aria-label="Duration value"
        />
      </label>
      <label
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "var(--sy-space-1)",
          fontSize: "0.8125rem",
          color: "var(--sy-color-fg-3)",
        }}
      >
        Unit
        <select
          value={action.durationUnit ?? "s"}
          onChange={(e) => onChange({ ...action, durationUnit: e.target.value as "s" | "min" | "h" })}
          style={{
            padding: "var(--sy-space-1) var(--sy-space-2)",
            borderRadius: "var(--sy-radius-sm)",
            border: "1px solid var(--sy-color-line)",
            background: "var(--sy-color-surface-1)",
            color: "var(--sy-color-fg)",
            fontSize: "0.875rem",
          }}
          aria-label="Duration unit"
        >
          <option value="s">seconds</option>
          <option value="min">minutes</option>
          <option value="h">hours</option>
        </select>
      </label>
    </div>
  );
}
