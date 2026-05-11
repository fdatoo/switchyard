import { useHomeAutomations, type AutomationItem } from "./hooks/useHomeAutomations";
import { Button } from "@/theme/primitives/button";

interface ActiveAutomationsSectionProps {
  /** Allow passing mock automations for testing */
  automations?: AutomationItem[];
}

/**
 * ActiveAutomationsSection — renders active/due automations with a "Run now" button.
 * "Run now" is a stub in Plan 02; Plan 10 will wire it to AutomationService.
 */
export function ActiveAutomationsSection({ automations }: ActiveAutomationsSectionProps) {
  const defaultAutomations = useHomeAutomations();
  const items = automations ?? defaultAutomations;

  function handleRunNow(id: string, name: string) {
    console.warn("TODO(plan-10): wire Run now to AutomationService", { id, name });
  }

  return (
    <section>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          marginBottom: "var(--sy-space-3)",
        }}
      >
        <h2
          style={{
            margin: 0,
            fontSize: "0.8125rem",
            fontWeight: 600,
            letterSpacing: "0.06em",
            textTransform: "uppercase",
            color: "var(--sy-color-fg-4)",
          }}
        >
          Active Automations
        </h2>
        <a
          href="/automations"
          style={{
            fontSize: "0.8125rem",
            color: "var(--sy-color-accent)",
            textDecoration: "none",
            fontWeight: 500,
          }}
        >
          All automations →
        </a>
      </div>
      <div
        style={{
          background: "var(--sy-color-surface-1)",
          border: "1px solid var(--sy-color-line)",
          borderRadius: "var(--sy-radius-lg)",
          overflow: "hidden",
        }}
      >
        {items.map((item, idx) => (
          <div
            key={item.id}
            data-testid="automation-row"
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: "var(--sy-space-3)",
              padding: "var(--sy-space-3) var(--sy-space-4)",
              borderTop: idx === 0 ? "none" : "1px solid var(--sy-color-line-soft)",
            }}
          >
            <div
              style={{
                flex: 1,
                minWidth: 0,
              }}
            >
              <div
                style={{
                  fontSize: "0.875rem",
                  fontWeight: 500,
                  color: "var(--sy-color-fg)",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {item.name}
              </div>
              <div
                style={{
                  fontSize: "0.75rem",
                  color: "var(--sy-color-fg-4)",
                  marginTop: "1px",
                }}
              >
                {item.timeLabel}
              </div>
            </div>
            <Button
              variant="secondary"
              onClick={() => handleRunNow(item.id, item.name)}
              style={{
                fontSize: "0.75rem",
                padding: "var(--sy-space-1) var(--sy-space-3)",
                flexShrink: 0,
              }}
            >
              Run now
            </Button>
          </div>
        ))}
      </div>
    </section>
  );
}
