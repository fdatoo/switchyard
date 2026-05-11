/**
 * EntityPicker — a select for one or more entity IDs.
 * TODO(plan-10 wire): replace mock entity list with ConfigService.GetArtifact.
 */

interface EntityPickerProps {
  value: string | string[];
  onChange: (value: string | string[]) => void;
  multi?: boolean;
  filter?: (id: string) => boolean;
  label?: string;
}

// Mock entity list — replace with ConfigService.GetArtifact in a future iteration.
const MOCK_ENTITIES = [
  "light.living_room_ceiling",
  "light.kitchen",
  "light.hallway",
  "light.bedroom",
  "scene.evening",
  "scene.morning",
  "notify.phone",
  "binary_sensor.motion",
  "sensor.lux",
];

export function EntityPicker({
  value,
  onChange,
  multi = false,
  filter,
  label = "Entity",
}: EntityPickerProps) {
  const entities = filter ? MOCK_ENTITIES.filter(filter) : MOCK_ENTITIES;
  const currentValue = Array.isArray(value) ? value : value ? [value] : [];

  if (multi) {
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
        {label}
        <select
          multiple
          value={currentValue}
          onChange={(e) => {
            const selected = Array.from(e.target.selectedOptions).map((o) => o.value);
            onChange(selected);
          }}
          style={{
            padding: "var(--sy-space-1) var(--sy-space-2)",
            borderRadius: "var(--sy-radius-sm)",
            border: "1px solid var(--sy-color-line)",
            background: "var(--sy-color-surface-1)",
            color: "var(--sy-color-fg)",
            fontSize: "0.875rem",
            minHeight: "6rem",
          }}
        >
          {entities.map((e) => (
            <option key={e} value={e}>
              {e}
            </option>
          ))}
        </select>
      </label>
    );
  }

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
      {label}
      <select
        value={currentValue[0] ?? ""}
        onChange={(e) => onChange(e.target.value)}
        style={{
          padding: "var(--sy-space-1) var(--sy-space-2)",
          borderRadius: "var(--sy-radius-sm)",
          border: "1px solid var(--sy-color-line)",
          background: "var(--sy-color-surface-1)",
          color: "var(--sy-color-fg)",
          fontSize: "0.875rem",
        }}
      >
        <option value="">— select entity —</option>
        {entities.map((e) => (
          <option key={e} value={e}>
            {e}
          </option>
        ))}
      </select>
    </label>
  );
}
