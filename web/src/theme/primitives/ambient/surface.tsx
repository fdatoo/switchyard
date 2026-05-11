import type { SurfaceProps } from "../surface";

/**
 * Ambient Surface primitive — glassmorphic card variant.
 * Uses --sy-* tokens only.
 */
export function AmbientSurface({
  children,
  style,
  as: Tag = "div",
  ...rest
}: SurfaceProps) {
  return (
    <Tag
      data-primitive="ambient-surface"
      style={{
        background: "color-mix(in srgb, var(--sy-color-surface-1) 55%, transparent)",
        borderRadius: "var(--sy-radius-xl)",
        border: "1px solid color-mix(in srgb, var(--sy-color-line) 30%, transparent)",
        backdropFilter: "blur(20px) saturate(1.4)",
        WebkitBackdropFilter: "blur(20px) saturate(1.4)",
        boxShadow: "var(--sy-shadow-2)",
        ...style,
      }}
      {...rest}
    >
      {children}
    </Tag>
  );
}
