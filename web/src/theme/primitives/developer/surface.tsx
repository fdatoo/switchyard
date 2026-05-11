import type { CSSProperties, ElementType, HTMLAttributes, ReactNode } from "react";

export interface DeveloperSurfaceProps extends HTMLAttributes<HTMLElement> {
  children?: ReactNode;
  style?: CSSProperties;
  as?: ElementType;
}

/**
 * Developer Surface primitive.
 * Near-black surface, 4px radius, 1px line border — using --sy-* tokens only.
 */
export function DeveloperSurface({
  children,
  style,
  as: Tag = "div",
  ...props
}: DeveloperSurfaceProps) {
  return (
    <Tag
      data-variant="developer-surface"
      style={{
        background: "var(--sy-color-surface-1)",
        borderRadius: "var(--sy-radius)",
        border: "1px solid var(--sy-color-line)",
        boxShadow: "var(--sy-shadow)",
        ...style,
      }}
      {...props}
    >
      {children}
    </Tag>
  );
}
