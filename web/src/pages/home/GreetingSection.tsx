import { useHomeGreeting } from "./hooks/useHomeGreeting";

interface GreetingSectionProps {
  alertCount: number;
}

/**
 * GreetingSection — displays the time-of-day greeting and calm/alert status line.
 * No interaction affordances; server-curated display only.
 */
export function GreetingSection({ alertCount }: GreetingSectionProps) {
  const { greeting, statusLine } = useHomeGreeting({ alertCount });

  return (
    <h1
      style={{
        margin: 0,
        fontFamily: "var(--sy-font-display)",
        fontSize: "1.75rem",
        fontWeight: 700,
        letterSpacing: "-0.02em",
        color: "var(--sy-color-fg)",
        lineHeight: 1.2,
      }}
    >
      {greeting}{" "}
      <span
        style={{
          fontWeight: 400,
          color: "var(--sy-color-fg-3)",
        }}
      >
        {statusLine}
      </span>
    </h1>
  );
}
