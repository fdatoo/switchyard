import type { ActionDraft } from "../useAutomationEditor";

interface RunScriptActionProps {
  action: ActionDraft;
  onChange: (action: ActionDraft) => void;
}

// Mock script list — replace with ScriptService.List in a future iteration.
const MOCK_SCRIPTS = ["lock-doors", "morning-scene", "evening-routine"];

export function RunScriptAction({ action, onChange }: RunScriptActionProps) {
  return (
    <label
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "var(--sy-space-1)",
        fontSize: "0.8125rem",
        color: "var(--sy-color-fg-3)",
      }}
    >
      Script
      <select
        value={action.scriptName ?? ""}
        onChange={(e) => onChange({ ...action, scriptName: e.target.value })}
        style={{
          padding: "var(--sy-space-1) var(--sy-space-2)",
          borderRadius: "var(--sy-radius-sm)",
          border: "1px solid var(--sy-color-line)",
          background: "var(--sy-color-surface-1)",
          color: "var(--sy-color-fg)",
          fontSize: "0.875rem",
        }}
      >
        <option value="">— select script —</option>
        {MOCK_SCRIPTS.map((s) => (
          <option key={s} value={s}>
            {s}
          </option>
        ))}
      </select>
    </label>
  );
}
