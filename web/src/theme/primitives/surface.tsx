import type { CSSProperties, ElementType, HTMLAttributes, ReactNode } from "react";

export interface SurfaceProps extends HTMLAttributes<HTMLElement> {
  children?: ReactNode;
  style?: CSSProperties;
  as?: ElementType;
}

/**
 * Friendly Surface primitive.
 * Renders a surface card using --sy-* tokens only.
 */
export function Surface({ children, style, as: Tag = "div", ...rest }: SurfaceProps) {
  return (
    <Tag
      style={{
        background: "var(--sy-color-surface-1)",
        borderRadius: "var(--sy-radius)",
        boxShadow: "var(--sy-shadow)",
        ...style,
      }}
      {...rest}
    >
      {children}
    </Tag>
  );
}
