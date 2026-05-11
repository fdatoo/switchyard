import type { ButtonHTMLAttributes } from "react";

export interface DeveloperButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "ghost";
  children?: React.ReactNode;
}

/**
 * Developer Button primitive.
 * Sharp corners (3-5px), monospace-aware, dense padding — using --sy-* tokens only.
 */
export function DeveloperButton({
  variant = "primary",
  children,
  style,
  ...props
}: DeveloperButtonProps) {
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
      data-variant="developer-button"
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: "var(--sy-space-2)",
        padding: "var(--sy-space-1) var(--sy-space-3)",
        borderRadius: "var(--sy-radius-sm)",
        background: bg,
        color,
        border:
          variant === "secondary"
            ? "1px solid var(--sy-color-line)"
            : variant === "ghost"
              ? "1px solid transparent"
              : "none",
        cursor: "pointer",
        font: "inherit",
        fontFamily: "var(--sy-font-numeric)",
        fontSize: "0.8125rem",
        fontWeight: 500,
        letterSpacing: "0.01em",
        transition: "var(--sy-motion-fast)",
        ...style,
      }}
      {...props}
    >
      {children}
    </button>
  );
}
