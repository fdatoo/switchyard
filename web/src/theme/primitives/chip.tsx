import type { HTMLAttributes } from "react";

export interface ChipProps extends HTMLAttributes<HTMLSpanElement> {
  active?: boolean;
}

/**
 * Friendly Chip primitive.
 * Small token/tag element using --sy-* tokens only.
 */
export function Chip({ active = false, children, style, ...rest }: ChipProps) {
  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        padding: "var(--sy-space-1) var(--sy-space-3)",
        borderRadius: "var(--sy-radius-pill)",
        fontSize: "0.71875rem",
        fontWeight: 500,
        background: active ? "var(--sy-color-accent)" : "var(--sy-color-surface-2)",
        color: active ? "var(--sy-color-bg)" : "var(--sy-color-fg-3)",
        border: active ? "1px solid var(--sy-color-accent)" : "1px solid var(--sy-color-line)",
        transition: "var(--sy-motion-fast)",
        ...style,
      }}
      {...rest}
    >
      {children}
    </span>
  );
}
