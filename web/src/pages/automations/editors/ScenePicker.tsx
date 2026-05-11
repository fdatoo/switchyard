import { EntityPicker } from "./EntityPicker";

interface ScenePickerProps {
  value: string;
  onChange: (value: string) => void;
  label?: string;
}

export function ScenePicker({ value, onChange, label = "Scene" }: ScenePickerProps) {
  return (
    <EntityPicker
      value={value}
      onChange={(v) => onChange(Array.isArray(v) ? v[0] ?? "" : v)}
      filter={(id) => id.startsWith("scene.")}
      label={label}
    />
  );
}
