import type { PillProps, PillVariant } from "../pill";

const ambientPillColors: Record<PillVariant, { bg: string; color: string }> = {
  accent: {
    bg: "color-mix(in srgb, var(--sy-color-accent) 40%, transparent)",
    color: "var(--sy-color-accent-2)",
  },
  good: {
    bg: "color-mix(in srgb, var(--sy-color-good) 30%, transparent)",
    color: "var(--sy-color-good)",
  },
  warn: {
    bg: "color-mix(in srgb, var(--sy-color-warn) 30%, transparent)",
    color: "var(--sy-color-warn)",
  },
  bad: {
    bg: "color-mix(in srgb, var(--sy-color-bad) 30%, transparent)",
    color: "var(--sy-color-bad)",
  },
  info: {
    bg: "color-mix(in srgb, var(--sy-color-info) 30%, transparent)",
    color: "var(--sy-color-info)",
  },
  neutral: {
    bg: "color-mix(in srgb, var(--sy-color-surface-1) 55%, transparent)",
    color: "var(--sy-color-fg-3)",
  },
};

/**
 * Ambient Pill primitive — glassmorphic status badge.
 * Uses --sy-* tokens only.
 */
export function AmbientPill({ variant = "neutral", children, style, ...rest }: PillProps) {
  const { bg, color } = ambientPillColors[variant];
  return (
    <span
      data-primitive="ambient-pill"
      style={{
        display: "inline-flex",
        alignItems: "center",
        padding: "var(--sy-space-1) var(--sy-space-3)",
        borderRadius: "var(--sy-radius-pill)",
        fontSize: "0.71875rem",
        fontWeight: 500,
        background: bg,
        color,
        border: "1px solid color-mix(in srgb, var(--sy-color-line) 30%, transparent)",
        backdropFilter: "blur(20px) saturate(1.4)",
        WebkitBackdropFilter: "blur(20px) saturate(1.4)",
        ...style,
      }}
      {...rest}
    >
      {children}
    </span>
  );
}
