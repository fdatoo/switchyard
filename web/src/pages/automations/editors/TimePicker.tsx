interface TimePickerProps {
  value: string;
  onChange: (value: string) => void;
  label?: string;
}

export function TimePicker({ value, onChange, label = "Time" }: TimePickerProps) {
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
      <input
        type="time"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        style={{
          padding: "var(--sy-space-1) var(--sy-space-2)",
          borderRadius: "var(--sy-radius-sm)",
          border: "1px solid var(--sy-color-line)",
          background: "var(--sy-color-surface-1)",
          color: "var(--sy-color-fg)",
          fontSize: "0.875rem",
          fontFamily: "var(--sy-font-mono, monospace)",
        }}
      />
    </label>
  );
}
