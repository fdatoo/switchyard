/**
 * TriggerSection — handles one automation trigger (single-trigger v2 policy).
 *
 * Five trigger types: SunEvent, Time, EntityStateChange, Webhook, Manual.
 * If multiple triggers detected, renders a read-only banner.
 */

import { useState } from "react";
import type { TriggerDraft, TriggerType } from "../useAutomationEditor";
import { TimePicker } from "../editors/TimePicker";
import { EntityPicker } from "../editors/EntityPicker";

interface TriggerSectionProps {
  trigger: TriggerDraft | null;
  onChange: (trigger: TriggerDraft | null) => void;
  multipleTriggersDetected?: boolean;
}

const TRIGGER_TYPE_LABELS: Record<TriggerType, string> = {
  SunEvent: "Sun event",
  Time: "Time",
  EntityStateChange: "Entity state change",
  Webhook: "Webhook",
  Manual: "Manual",
};

export function TriggerSection({
  trigger,
  onChange,
  multipleTriggersDetected = false,
}: TriggerSectionProps) {
  const [collapsed, setCollapsed] = useState(false);

  const summary = trigger
    ? `${TRIGGER_TYPE_LABELS[trigger.type]}${trigger.sunEvent ? ` — ${trigger.sunEvent}` : ""}${trigger.timeAt ? ` — ${trigger.timeAt}` : ""}`
    : "No trigger configured";

  function handleTypeChange(type: TriggerType) {
    onChange({ type });
  }

  return (
    <section
      style={{
        background: "var(--sy-color-surface-1)",
        borderRadius: "var(--sy-radius)",
        border: "1px solid var(--sy-color-line-soft)",
        overflow: "hidden",
      }}
    >
      <button
        type="button"
        onClick={() => setCollapsed((c) => !c)}
        style={{
          width: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "var(--sy-space-3) var(--sy-space-4)",
          background: "none",
          border: "none",
          cursor: "pointer",
          color: "var(--sy-color-fg)",
          fontSize: "0.9375rem",
          fontWeight: 600,
          textAlign: "left",
        }}
      >
        <span>Trigger</span>
        {collapsed && (
          <span
            style={{
              fontSize: "0.8125rem",
              fontWeight: 400,
              color: "var(--sy-color-fg-4)",
              padding: "2px var(--sy-space-2)",
              borderRadius: "var(--sy-radius-pill)",
              background: "var(--sy-color-surface-2)",
            }}
          >
            {summary}
          </span>
        )}
        <span
          style={{
            fontSize: "0.75rem",
            color: "var(--sy-color-fg-4)",
            transform: collapsed ? "rotate(-90deg)" : "rotate(0deg)",
            transition: "var(--sy-motion-fast)",
          }}
        >
          ▼
        </span>
      </button>

      {!collapsed && (
        <div
          style={{
            padding: "0 var(--sy-space-4) var(--sy-space-4)",
            display: "flex",
            flexDirection: "column",
            gap: "var(--sy-space-3)",
          }}
        >
          {multipleTriggersDetected ? (
            <div
              role="note"
              aria-label="Multiple triggers detected"
              style={{
                padding: "var(--sy-space-2) var(--sy-space-3)",
                background: "var(--sy-color-surface-2)",
                borderRadius: "var(--sy-radius-sm)",
                border: "1px solid var(--sy-color-warn)",
                fontSize: "0.8125rem",
                color: "var(--sy-color-fg-3)",
                display: "flex",
                alignItems: "center",
                gap: "var(--sy-space-2)",
              }}
            >
              Multiple triggers —{" "}
              <a href="/_authed/pkl-editor" style={{ color: "var(--sy-color-accent)", textDecoration: "none" }}>
                edit in Pkl editor →
              </a>
            </div>
          ) : (
            <>
              <label
                style={{
                  display: "flex",
                  flexDirection: "column",
                  gap: "var(--sy-space-1)",
                  fontSize: "0.8125rem",
                  color: "var(--sy-color-fg-3)",
                }}
              >
                Trigger type
                <select
                  value={trigger?.type ?? "Manual"}
                  onChange={(e) => handleTypeChange(e.target.value as TriggerType)}
                  style={{
                    padding: "var(--sy-space-1) var(--sy-space-2)",
                    borderRadius: "var(--sy-radius-sm)",
                    border: "1px solid var(--sy-color-line)",
                    background: "var(--sy-color-surface-1)",
                    color: "var(--sy-color-fg)",
                    fontSize: "0.875rem",
                  }}
                >
                  {(Object.keys(TRIGGER_TYPE_LABELS) as TriggerType[]).map((t) => (
                    <option key={t} value={t}>
                      {TRIGGER_TYPE_LABELS[t]}
                    </option>
                  ))}
                </select>
              </label>

              {trigger?.type === "SunEvent" && (
                <label
                  style={{
                    display: "flex",
                    flexDirection: "column",
                    gap: "var(--sy-space-1)",
                    fontSize: "0.8125rem",
                    color: "var(--sy-color-fg-3)",
                  }}
                >
                  Sun event
                  <select
                    value={trigger.sunEvent ?? "sunset"}
                    onChange={(e) => onChange({ ...trigger, sunEvent: e.target.value as "sunrise" | "sunset" })}
                    style={{
                      padding: "var(--sy-space-1) var(--sy-space-2)",
                      borderRadius: "var(--sy-radius-sm)",
                      border: "1px solid var(--sy-color-line)",
                      background: "var(--sy-color-surface-1)",
                      color: "var(--sy-color-fg)",
                      fontSize: "0.875rem",
                    }}
                  >
                    <option value="sunrise">Sunrise</option>
                    <option value="sunset">Sunset</option>
                  </select>
                </label>
              )}

              {trigger?.type === "Time" && (
                <>
                  {!trigger.useCron ? (
                    <TimePicker
                      value={trigger.timeAt ?? ""}
                      onChange={(v) => onChange({ ...trigger, timeAt: v })}
                    />
                  ) : (
                    <label
                      style={{
                        display: "flex",
                        flexDirection: "column",
                        gap: "var(--sy-space-1)",
                        fontSize: "0.8125rem",
                        color: "var(--sy-color-fg-3)",
                      }}
                    >
                      Cron expression
                      <input
                        type="text"
                        value={trigger.cron ?? ""}
                        onChange={(e) => onChange({ ...trigger, cron: e.target.value })}
                        placeholder="0 21 * * *"
                        style={{
                          padding: "var(--sy-space-1) var(--sy-space-2)",
                          borderRadius: "var(--sy-radius-sm)",
                          border: "1px solid var(--sy-color-line)",
                          background: "var(--sy-color-surface-1)",
                          color: "var(--sy-color-fg)",
                          fontSize: "0.875rem",
                          fontFamily: "var(--sy-font-mono, monospace)",
                        }}
                      />
                    </label>
                  )}
                  <label
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: "var(--sy-space-2)",
                      fontSize: "0.8125rem",
                      color: "var(--sy-color-fg-3)",
                      cursor: "pointer",
                    }}
                  >
                    <input
                      type="checkbox"
                      checked={trigger.useCron ?? false}
                      onChange={(e) => onChange({ ...trigger, useCron: e.target.checked })}
                    />
                    Advanced: use cron
                  </label>
                </>
              )}

              {trigger?.type === "EntityStateChange" && (
                <>
                  <EntityPicker
                    value={trigger.entities ?? []}
                    onChange={(v) => onChange({ ...trigger, entities: Array.isArray(v) ? v : [v] })}
                    multi
                    label="Entities"
                  />
                  <div
                    style={{
                      display: "flex",
                      gap: "var(--sy-space-2)",
                    }}
                  >
                    <label
                      style={{
                        display: "flex",
                        flexDirection: "column",
                        gap: "var(--sy-space-1)",
                        fontSize: "0.8125rem",
                        color: "var(--sy-color-fg-3)",
                        flex: 1,
                      }}
                    >
                      From (optional)
                      <input
                        type="text"
                        value={trigger.from ?? ""}
                        onChange={(e) => onChange({ ...trigger, from: e.target.value })}
                        placeholder="any state"
                        style={{
                          padding: "var(--sy-space-1) var(--sy-space-2)",
                          borderRadius: "var(--sy-radius-sm)",
                          border: "1px solid var(--sy-color-line)",
                          background: "var(--sy-color-surface-1)",
                          color: "var(--sy-color-fg)",
                          fontSize: "0.875rem",
                        }}
                      />
                    </label>
                    <label
                      style={{
                        display: "flex",
                        flexDirection: "column",
                        gap: "var(--sy-space-1)",
                        fontSize: "0.8125rem",
                        color: "var(--sy-color-fg-3)",
                        flex: 1,
                      }}
                    >
                      To (optional)
                      <input
                        type="text"
                        value={trigger.to ?? ""}
                        onChange={(e) => onChange({ ...trigger, to: e.target.value })}
                        placeholder="any state"
                        style={{
                          padding: "var(--sy-space-1) var(--sy-space-2)",
                          borderRadius: "var(--sy-radius-sm)",
                          border: "1px solid var(--sy-color-line)",
                          background: "var(--sy-color-surface-1)",
                          color: "var(--sy-color-fg)",
                          fontSize: "0.875rem",
                        }}
                      />
                    </label>
                  </div>
                </>
              )}

              {trigger?.type === "Webhook" && (
                <>
                  <label
                    style={{
                      display: "flex",
                      flexDirection: "column",
                      gap: "var(--sy-space-1)",
                      fontSize: "0.8125rem",
                      color: "var(--sy-color-fg-3)",
                    }}
                  >
                    Path
                    <input
                      type="text"
                      value={trigger.path ?? ""}
                      onChange={(e) => onChange({ ...trigger, path: e.target.value })}
                      placeholder="/hooks/my-webhook"
                      style={{
                        padding: "var(--sy-space-1) var(--sy-space-2)",
                        borderRadius: "var(--sy-radius-sm)",
                        border: "1px solid var(--sy-color-line)",
                        background: "var(--sy-color-surface-1)",
                        color: "var(--sy-color-fg)",
                        fontSize: "0.875rem",
                        fontFamily: "var(--sy-font-mono, monospace)",
                      }}
                    />
                  </label>
                  <fieldset
                    style={{
                      border: "1px solid var(--sy-color-line)",
                      borderRadius: "var(--sy-radius-sm)",
                      padding: "var(--sy-space-2) var(--sy-space-3)",
                    }}
                  >
                    <legend style={{ fontSize: "0.8125rem", color: "var(--sy-color-fg-3)" }}>Methods</legend>
                    {["GET", "POST", "PUT"].map((m) => (
                      <label
                        key={m}
                        style={{
                          display: "inline-flex",
                          alignItems: "center",
                          gap: "var(--sy-space-1)",
                          marginRight: "var(--sy-space-3)",
                          fontSize: "0.8125rem",
                          color: "var(--sy-color-fg)",
                          cursor: "pointer",
                        }}
                      >
                        <input
                          type="checkbox"
                          checked={(trigger.methods ?? ["POST"]).includes(m)}
                          onChange={(e) => {
                            const methods = trigger.methods ?? ["POST"];
                            onChange({
                              ...trigger,
                              methods: e.target.checked
                                ? [...methods, m]
                                : methods.filter((x) => x !== m),
                            });
                          }}
                        />
                        {m}
                      </label>
                    ))}
                  </fieldset>
                </>
              )}

              {trigger?.type === "Manual" && (
                <p
                  style={{
                    margin: 0,
                    fontSize: "0.8125rem",
                    color: "var(--sy-color-fg-4)",
                    fontStyle: "italic",
                  }}
                >
                  Runs only when triggered manually or via Run now.
                </p>
              )}
            </>
          )}
        </div>
      )}
    </section>
  );
}
