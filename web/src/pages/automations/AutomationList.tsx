/**
 * AutomationList — displays all automations with links to their editor.
 *
 * Row layout (CSS grid: 36px 1fr auto auto):
 *  1. Left icon: AutomationsIcon in an accent-soft square
 *  2. Name + trigger summary (two lines)
 *  3. Status pill: green "active" or muted "disabled"
 *  4. "Run now" action button
 *
 * TODO(plan-10 wire): replace useAutomations() mock with AutomationService.List RPC.
 */

import { AutomationsIcon } from "@/shell/icons";
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
      <header>
        <h1
          style={{
            margin: 0,
            fontSize: "1.75rem",
            fontWeight: 600,
            color: "var(--sy-color-fg)",
            letterSpacing: "-0.02em",
          }}
        >
          Automations
        </h1>
        <p
          style={{
            margin: "var(--sy-space-1) 0 0",
            color: "var(--sy-color-fg-3)",
            fontSize: "0.9375rem",
          }}
        >
          {automations.length === 0
            ? "No automations configured."
            : `${automations.length} ${automations.length === 1 ? "automation" : "automations"}`}
        </p>
      </header>

      {automations.length === 0 ? (
        <div
          style={{
            background: "var(--sy-color-surface-1)",
            borderRadius: "var(--sy-radius-lg)",
            boxShadow: "var(--sy-shadow)",
            padding: "var(--sy-space-5)",
            textAlign: "center",
            color: "var(--sy-color-fg-3)",
          }}
        >
          <p style={{ margin: 0 }}>
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
        </div>
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
                display: "grid",
                gridTemplateColumns: "36px 1fr auto auto",
                gap: "var(--sy-space-3)",
                alignItems: "center",
                padding: "var(--sy-space-3) var(--sy-space-4)",
                background: "var(--sy-color-surface-1)",
                borderRadius: "var(--sy-radius-lg)",
                boxShadow: "var(--sy-shadow)",
              }}
            >
              {/* 1. Icon */}
              <span
                style={{
                  display: "inline-flex",
                  alignItems: "center",
                  justifyContent: "center",
                  width: "32px",
                  height: "32px",
                  borderRadius: "var(--sy-radius)",
                  background: "var(--sy-color-accent-soft)",
                  color: "var(--sy-color-accent)",
                  flexShrink: 0,
                }}
              >
                <AutomationsIcon size={16} />
              </span>

              {/* 2. Name + trigger summary */}
              <a
                href={`/_authed/automations/${encodeURIComponent(a.id)}`}
                style={{
                  textDecoration: "none",
                  color: "var(--sy-color-fg)",
                  minWidth: 0,
                }}
              >
                <div
                  style={{
                    fontWeight: 600,
                    fontSize: "0.9375rem",
                    letterSpacing: "-0.005em",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {a.displayName || a.id}
                </div>
                <div
                  style={{
                    fontSize: "0.8125rem",
                    color: "var(--sy-color-fg-3)",
                    marginTop: "2px",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {a.trigger.summary}
                </div>
              </a>

              {/* 3. Status pill */}
              <Chip
                style={
                  a.enabled
                    ? {
                        background: "color-mix(in srgb, var(--sy-color-good) 12%, transparent)",
                        color: "var(--sy-color-good)",
                        borderColor: "color-mix(in srgb, var(--sy-color-good) 30%, transparent)",
                      }
                    : {
                        background: "var(--sy-color-surface-2)",
                        color: "var(--sy-color-fg-4)",
                        borderColor: "var(--sy-color-line)",
                      }
                }
              >
                {a.enabled ? "active" : "disabled"}
              </Chip>

              {/* 4. Run now button */}
              <button
                type="button"
                aria-label={`Run ${a.displayName || a.id} now`}
                style={{
                  display: "inline-flex",
                  alignItems: "center",
                  padding: "var(--sy-space-1) var(--sy-space-3)",
                  borderRadius: "var(--sy-radius-pill)",
                  border: "1px solid var(--sy-color-line)",
                  background: "var(--sy-color-surface-2)",
                  color: "var(--sy-color-fg-2)",
                  fontSize: "0.75rem",
                  fontWeight: 500,
                  cursor: "pointer",
                  whiteSpace: "nowrap",
                }}
                onClick={(e) => {
                  e.stopPropagation();
                  // TODO(plan-10 wire): call AutomationService.RunNow RPC
                }}
              >
                Run now
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
