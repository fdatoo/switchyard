interface PlaceholderPageProps {
  title: string;
  plan: string;
}

/**
 * Shared placeholder component for pages not yet built.
 * Used by every v2 route until its plan ships.
 */
export function PlaceholderPage({ title, plan }: PlaceholderPageProps) {
  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "var(--sy-space-2)",
        padding: "var(--sy-space-5) var(--sy-space-6)",
      }}
    >
      <h1
        style={{
          margin: 0,
          fontSize: "1.5rem",
          fontWeight: 600,
          color: "var(--sy-color-fg)",
          letterSpacing: "-0.018em",
        }}
      >
        {title}
      </h1>
      <p
        style={{
          margin: 0,
          fontSize: "0.8125rem",
          color: "var(--sy-color-fg-4)",
          fontStyle: "italic",
        }}
      >
        Coming in {plan}
      </p>
    </div>
  );
}
