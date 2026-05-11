import type { ChipProps } from "../chip";

/**
 * Ambient Chip primitive — glassmorphic capsule variant.
 * Uses --sy-* tokens only.
 */
export function AmbientChip({ active = false, children, style, ...rest }: ChipProps) {
  return (
    <span
      data-primitive="ambient-chip"
      style={{
        display: "inline-flex",
        alignItems: "center",
        padding: "var(--sy-space-1) var(--sy-space-3)",
        borderRadius: "var(--sy-radius-pill)",
        fontSize: "0.71875rem",
        fontWeight: 500,
        background: active
          ? "color-mix(in srgb, var(--sy-color-accent) 60%, transparent)"
          : "color-mix(in srgb, var(--sy-color-surface-1) 55%, transparent)",
        color: active ? "var(--sy-color-fg)" : "var(--sy-color-fg-3)",
        border: "1px solid color-mix(in srgb, var(--sy-color-line) 30%, transparent)",
        backdropFilter: "blur(20px) saturate(1.4)",
        WebkitBackdropFilter: "blur(20px) saturate(1.4)",
        transition: "var(--sy-motion-fast)",
        ...style,
      }}
      {...rest}
    >
      {children}
    </span>
  );
}
