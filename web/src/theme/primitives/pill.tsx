import type { HTMLAttributes } from "react";

export type PillVariant = "accent" | "good" | "warn" | "bad" | "info" | "neutral";

export interface PillProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: PillVariant;
}

const pillColors: Record<PillVariant, { bg: string; color: string }> = {
  accent: { bg: "var(--sy-color-accent-soft)", color: "var(--sy-color-accent)" },
  good: { bg: "color-mix(in oklch, var(--sy-color-good) 15%, transparent)", color: "var(--sy-color-good)" },
  warn: { bg: "color-mix(in oklch, var(--sy-color-warn) 15%, transparent)", color: "var(--sy-color-warn)" },
  bad: { bg: "color-mix(in oklch, var(--sy-color-bad) 15%, transparent)", color: "var(--sy-color-bad)" },
  info: { bg: "color-mix(in oklch, var(--sy-color-info) 15%, transparent)", color: "var(--sy-color-info)" },
  neutral: { bg: "var(--sy-color-surface-2)", color: "var(--sy-color-fg-3)" },
};

/**
 * Friendly Pill primitive.
 * Status/label badge using --sy-* tokens only.
 */
export function Pill({ variant = "neutral", children, style, ...rest }: PillProps) {
  const { bg, color } = pillColors[variant];
  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        padding: "1px var(--sy-space-2)",
        borderRadius: "var(--sy-radius-pill)",
        fontSize: "0.65625rem",
        fontWeight: 500,
        background: bg,
        color,
        ...style,
      }}
      {...rest}
    >
      {children}
    </span>
  );
}
