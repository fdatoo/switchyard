import type { ButtonHTMLAttributes } from "react";
import type { ButtonProps } from "../button";

/**
 * Ambient Button primitive — glassmorphic capsule variant.
 * Uses --sy-* tokens only.
 */
export function AmbientButton({
  variant = "primary",
  children,
  style,
  ...rest
}: ButtonProps & ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      data-primitive="ambient-button"
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: "var(--sy-space-2)",
        padding: "var(--sy-space-2) var(--sy-space-4)",
        borderRadius: "var(--sy-radius-xl)",
        background: variant === "primary"
          ? "color-mix(in srgb, var(--sy-color-accent) 70%, transparent)"
          : "color-mix(in srgb, var(--sy-color-surface-1) 55%, transparent)",
        color: variant === "primary" ? "var(--sy-color-fg)" : "var(--sy-color-fg-2)",
        border: "1px solid color-mix(in srgb, var(--sy-color-line) 30%, transparent)",
        backdropFilter: "blur(20px) saturate(1.4)",
        WebkitBackdropFilter: "blur(20px) saturate(1.4)",
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
