/**
 * AutomationList — displays all automations with links to their editor.
 *
 * TODO(plan-10 wire): replace useAutomations() mock with AutomationService.List RPC.
 */

import { Chip } from "@/theme/primitives/chip";
import { useAutomations } from "./useAutomations";

export function AutomationList() {
  const automations = useAutomations();

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "var(--sy-space-4)",
        padding: "var(--sy-space-5) var(--sy-space-6)",
        maxWidth: "900px",
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
        Automations
      </h1>

      {automations.length === 0 ? (
        <p
          style={{
            margin: 0,
            fontSize: "0.875rem",
            color: "var(--sy-color-fg-4)",
            fontStyle: "italic",
          }}
        >
          No automations yet. Create one in{" "}
          <code
            style={{
              fontFamily: "var(--sy-font-mono, monospace)",
              fontSize: "0.8125rem",
              background: "var(--sy-color-surface-2)",
              borderRadius: "var(--sy-radius-sm)",
              padding: "1px var(--sy-space-1)",
            }}
          >
            ~/.switchyard/automations/
          </code>
        </p>
      ) : (
        <ul
          role="list"
          style={{
            margin: 0,
            padding: 0,
            listStyle: "none",
            display: "flex",
            flexDirection: "column",
            gap: "var(--sy-space-2)",
          }}
        >
          {automations.map((a) => (
            <li
              key={a.id}
              style={{
                display: "flex",
                alignItems: "center",
                gap: "var(--sy-space-3)",
                padding: "var(--sy-space-3) var(--sy-space-4)",
                background: "var(--sy-color-surface-1)",
                borderRadius: "var(--sy-radius)",
                border: "1px solid var(--sy-color-line-soft)",
              }}
            >
              <a
                href={`/_authed/automations/${encodeURIComponent(a.id)}`}
                style={{
                  flex: 1,
                  textDecoration: "none",
                  color: "var(--sy-color-fg)",
                  fontSize: "0.9375rem",
                  fontWeight: 500,
                }}
              >
                {a.displayName || a.id}
              </a>
              {!a.enabled && (
                <Chip
                  style={{
                    background: "var(--sy-color-surface-2)",
                    color: "var(--sy-color-fg-4)",
                    borderColor: "var(--sy-color-line)",
                    fontSize: "0.71875rem",
                  }}
                >
                  disabled
                </Chip>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
