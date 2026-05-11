import type { HTMLAttributes } from "react";

export type DeveloperPillVariant = "accent" | "good" | "warn" | "bad" | "info" | "neutral";

export interface DeveloperPillProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: DeveloperPillVariant;
  children?: React.ReactNode;
}

const pillColors: Record<DeveloperPillVariant, { bg: string; color: string }> = {
  accent: { bg: "var(--sy-color-accent-subtle)", color: "var(--sy-color-accent)" },
  good: {
    bg: "color-mix(in oklch, var(--sy-color-good) 12%, transparent)",
    color: "var(--sy-color-good)",
  },
  warn: {
    bg: "color-mix(in oklch, var(--sy-color-warn) 12%, transparent)",
    color: "var(--sy-color-warn)",
  },
  bad: {
    bg: "color-mix(in oklch, var(--sy-color-bad) 12%, transparent)",
    color: "var(--sy-color-bad)",
  },
  info: {
    bg: "color-mix(in oklch, var(--sy-color-info) 12%, transparent)",
    color: "var(--sy-color-info)",
  },
  neutral: { bg: "var(--sy-color-surface-2)", color: "var(--sy-color-fg-3)" },
};

/**
 * Developer Pill primitive.
 * Flat rectangular status badge — sharp 3px corners, monospace font — using --sy-* tokens only.
 */
export function DeveloperPill({
  variant = "neutral",
  children,
  style,
  ...props
}: DeveloperPillProps) {
  const { bg, color } = pillColors[variant];
  return (
    <span
      data-variant="developer-pill"
      style={{
        display: "inline-flex",
        alignItems: "center",
        padding: "1px var(--sy-space-2)",
        borderRadius: "var(--sy-radius-sm)",
        fontSize: "0.625rem",
        fontFamily: "var(--sy-font-numeric)",
        fontWeight: 500,
        letterSpacing: "0.04em",
        textTransform: "uppercase",
        background: bg,
        color,
        border: "1px solid currentColor",
        opacity: 0.9,
        ...style,
      }}
      {...props}
    >
      {children}
    </span>
  );
}
