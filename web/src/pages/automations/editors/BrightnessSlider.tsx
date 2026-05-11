interface BrightnessSliderProps {
  value: number;
  onChange: (value: number) => void;
  label?: string;
}

export function BrightnessSlider({ value, onChange, label = "Brightness" }: BrightnessSliderProps) {
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
      <span>
        {label}:{" "}
        <span
          style={{
            color: "var(--sy-color-fg)",
            fontWeight: 500,
          }}
        >
          {value}%
        </span>
      </span>
      <input
        type="range"
        min={0}
        max={100}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        style={{
          accentColor: "var(--sy-color-accent)",
          cursor: "pointer",
        }}
        aria-label={`${label} ${value}%`}
      />
    </label>
  );
}
