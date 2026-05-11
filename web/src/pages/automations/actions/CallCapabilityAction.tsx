import type { ActionDraft } from "../useAutomationEditor";

interface CallCapabilityActionProps {
  action: ActionDraft;
  onChange: (action: ActionDraft) => void;
}

export function CallCapabilityAction({ action, onChange }: CallCapabilityActionProps) {
  return (
    <div style={{ display: "flex", gap: "var(--sy-space-2)", flexWrap: "wrap" }}>
      <label
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "var(--sy-space-1)",
          fontSize: "0.8125rem",
          color: "var(--sy-color-fg-3)",
          flex: 1,
          minWidth: "10rem",
        }}
      >
        Entity
        <input
          type="text"
          value={action.entity ?? ""}
          onChange={(e) => onChange({ ...action, entity: e.target.value })}
          placeholder="entity.id"
          style={{
            padding: "var(--sy-space-1) var(--sy-space-2)",
            borderRadius: "var(--sy-radius-sm)",
            border: "1px solid var(--sy-color-line)",
            background: "var(--sy-color-surface-1)",
            color: "var(--sy-color-fg)",
            fontSize: "0.875rem",
          }}
        />
      </label>
      <label
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "var(--sy-space-1)",
          fontSize: "0.8125rem",
          color: "var(--sy-color-fg-3)",
          flex: 1,
          minWidth: "10rem",
        }}
      >
        Capability
        <input
          type="text"
          value={action.capability ?? ""}
          onChange={(e) => onChange({ ...action, capability: e.target.value })}
          placeholder="turn_on"
          style={{
            padding: "var(--sy-space-1) var(--sy-space-2)",
            borderRadius: "var(--sy-radius-sm)",
            border: "1px solid var(--sy-color-line)",
            background: "var(--sy-color-surface-1)",
            color: "var(--sy-color-fg)",
            fontSize: "0.875rem",
          }}
        />
      </label>
    </div>
  );
}
