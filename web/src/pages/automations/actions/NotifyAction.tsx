import type { ActionDraft } from "../useAutomationEditor";
import { EntityPicker } from "../editors/EntityPicker";

interface NotifyActionProps {
  action: ActionDraft;
  onChange: (action: ActionDraft) => void;
}

export function NotifyAction({ action, onChange }: NotifyActionProps) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--sy-space-2)" }}>
      <EntityPicker
        value={action.entity ?? ""}
        onChange={(v) => onChange({ ...action, entity: Array.isArray(v) ? v[0] ?? "" : v })}
        filter={(id) => id.startsWith("notify.")}
        label="Notification target"
      />
      <label
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "var(--sy-space-1)",
          fontSize: "0.8125rem",
          color: "var(--sy-color-fg-3)",
        }}
      >
        Message
        <textarea
          value={action.message ?? ""}
          onChange={(e) => onChange({ ...action, message: e.target.value })}
          rows={3}
          placeholder="Notification message…"
          style={{
            padding: "var(--sy-space-2)",
            borderRadius: "var(--sy-radius-sm)",
            border: "1px solid var(--sy-color-line)",
            background: "var(--sy-color-surface-1)",
            color: "var(--sy-color-fg)",
            fontSize: "0.875rem",
            resize: "vertical",
          }}
        />
      </label>
    </div>
  );
}
