import type { ButtonHTMLAttributes } from "react";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "ghost";
}

/**
 * Friendly Button primitive.
 * Uses --sy-* tokens only — no raw colors, radii, or spacing.
 */
export function Button({ variant = "primary", children, style, ...rest }: ButtonProps) {
  const bg =
    variant === "primary"
      ? "var(--sy-color-accent)"
      : variant === "secondary"
        ? "var(--sy-color-surface-2)"
        : "transparent";

  const color =
    variant === "primary" ? "var(--sy-color-bg)" : "var(--sy-color-fg)";

  return (
    <button
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: "var(--sy-space-2)",
        padding: "var(--sy-space-2) var(--sy-space-4)",
        borderRadius: "var(--sy-radius)",
        background: bg,
        color,
        border: variant === "secondary" ? "1px solid var(--sy-color-line)" : "none",
        cursor: "pointer",
        font: "inherit",
        fontSize: "0.875rem",
        fontWeight: 500,
        transition: "var(--sy-motion-fast)",
        ...style,
      }}
      {...rest}
    >
      {children}
    </button>
  );
}
