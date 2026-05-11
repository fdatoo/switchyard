import type { ActionDraft } from "../useAutomationEditor";
import { EntityPicker } from "../editors/EntityPicker";

interface TurnOnActionProps {
  action: ActionDraft;
  onChange: (action: ActionDraft) => void;
}

export function TurnOnAction({ action, onChange }: TurnOnActionProps) {
  return (
    <EntityPicker
      value={action.entity ?? ""}
      onChange={(v) => onChange({ ...action, entity: Array.isArray(v) ? v[0] ?? "" : v })}
      label="Entity"
    />
  );
}
