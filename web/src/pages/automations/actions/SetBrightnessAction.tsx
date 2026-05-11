import type { ActionDraft } from "../useAutomationEditor";
import { EntityPicker } from "../editors/EntityPicker";
import { BrightnessSlider } from "../editors/BrightnessSlider";

interface SetBrightnessActionProps {
  action: ActionDraft;
  onChange: (action: ActionDraft) => void;
}

export function SetBrightnessAction({ action, onChange }: SetBrightnessActionProps) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--sy-space-2)" }}>
      <EntityPicker
        value={action.entity ?? ""}
        onChange={(v) => onChange({ ...action, entity: Array.isArray(v) ? v[0] ?? "" : v })}
        label="Entity"
      />
      <BrightnessSlider
        value={action.brightness ?? 50}
        onChange={(v) => onChange({ ...action, brightness: v })}
      />
    </div>
  );
}
