import type { ActionDraft } from "../useAutomationEditor";
import { ScenePicker } from "../editors/ScenePicker";

interface SceneActionProps {
  action: ActionDraft;
  onChange: (action: ActionDraft) => void;
}

export function SceneAction({ action, onChange }: SceneActionProps) {
  return (
    <ScenePicker
      value={action.sceneName ?? ""}
      onChange={(v) => onChange({ ...action, sceneName: v })}
    />
  );
}
