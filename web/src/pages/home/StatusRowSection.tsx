import { useHomeStatus, type StatusSeverity } from "./hooks/useHomeStatus";

const severityColorMap: Record<StatusSeverity, string> = {
  good: "var(--sy-color-good)",
  warn: "var(--sy-color-warn)",
  bad: "var(--sy-color-bad)",
  neutral: "var(--sy-color-fg-3)",
};

const severityBgMap: Record<StatusSeverity, string> = {
  good: "color-mix(in oklch, var(--sy-color-good) 15%, transparent)",
  warn: "color-mix(in oklch, var(--sy-color-warn) 15%, transparent)",
  bad: "color-mix(in oklch, var(--sy-color-bad) 15%, transparent)",
  neutral: "var(--sy-color-surface-2)",
};

interface StatusRowSectionProps {
  /** Allow passing mock items for testing */
  items?: ReturnType<typeof useHomeStatus>;
}

/**
 * StatusRowSection — renders a horizontal row of status pills.
 * Each pill color-codes by severity using --sy-color-* tokens.
 */
export function StatusRowSection({ items }: StatusRowSectionProps) {
  const defaultItems = useHomeStatus();
  const pillItems = items ?? defaultItems;

  return (
    <div
      style={{
        display: "flex",
        flexWrap: "wrap",
        gap: "var(--sy-space-2)",
        alignItems: "center",
      }}
    >
      {pillItems.map((item) => (
        <span
          key={item.id}
          data-severity={item.severity}
          style={{
            display: "inline-flex",
            alignItems: "center",
            padding: "2px var(--sy-space-3)",
            borderRadius: "var(--sy-radius-pill)",
            fontSize: "0.75rem",
            fontWeight: 500,
            background: severityBgMap[item.severity],
            color: severityColorMap[item.severity],
          }}
        >
          {item.label}
        </span>
      ))}
    </div>
  );
}
