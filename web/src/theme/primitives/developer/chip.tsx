import type { HTMLAttributes } from "react";

export interface DeveloperChipProps extends HTMLAttributes<HTMLSpanElement> {
  active?: boolean;
  children?: React.ReactNode;
}

/**
 * Developer Chip primitive.
 * 3px radius, tight horizontal padding, monospace font — using --sy-* tokens only.
 */
export function DeveloperChip({
  active = false,
  children,
  style,
  ...props
}: DeveloperChipProps) {
  return (
    <span
      data-variant="developer-chip"
      style={{
        display: "inline-flex",
        alignItems: "center",
        padding: "1px var(--sy-space-2)",
        borderRadius: "var(--sy-radius-sm)",
        fontSize: "0.6875rem",
        fontFamily: "var(--sy-font-numeric)",
        fontWeight: 500,
        letterSpacing: "0.02em",
        background: active ? "var(--sy-color-accent-subtle)" : "var(--sy-color-surface-2)",
        color: active ? "var(--sy-color-accent)" : "var(--sy-color-fg-3)",
        border: active
          ? "1px solid var(--sy-color-accent)"
          : "1px solid var(--sy-color-line)",
        transition: "var(--sy-motion-fast)",
        ...style,
      }}
      {...props}
    >
      {children}
    </span>
  );
}
