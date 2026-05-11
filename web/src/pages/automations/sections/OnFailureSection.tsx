/**
 * OnFailureSection — strategy selector for automation failure handling.
 *
 * Three strategies: "Do nothing" (ignore), "Retry", "Notify".
 * Conditional fields render below the selector.
 */

import { useState } from "react";
import type { OnFailureDraft, OnFailureStrategyType } from "../useAutomationEditor";

interface OnFailureSectionProps {
  onFailure: OnFailureDraft;
  onChange: (onFailure: OnFailureDraft) => void;
}

export function OnFailureSection({ onFailure, onChange }: OnFailureSectionProps) {
  const [collapsed, setCollapsed] = useState(false);

  const summaryLabels: Record<OnFailureStrategyType, string> = {
    ignore: "Do nothing",
    retry: "Retry",
    notify: "Notify",
  };

  const summary = summaryLabels[onFailure.strategy];

  function handleStrategyChange(strategy: OnFailureStrategyType) {
    onChange({ ...onFailure, strategy });
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
        <span>On failure</span>
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
          <label
            style={{
              display: "flex",
              flexDirection: "column",
              gap: "var(--sy-space-1)",
              fontSize: "0.8125rem",
              color: "var(--sy-color-fg-3)",
            }}
          >
            Strategy
            <select
              value={onFailure.strategy}
              onChange={(e) => handleStrategyChange(e.target.value as OnFailureStrategyType)}
              aria-label="On-failure strategy"
              style={{
                padding: "var(--sy-space-1) var(--sy-space-2)",
                borderRadius: "var(--sy-radius-sm)",
                border: "1px solid var(--sy-color-line)",
                background: "var(--sy-color-surface-1)",
                color: "var(--sy-color-fg)",
                fontSize: "0.875rem",
              }}
            >
              <option value="ignore">Do nothing</option>
              <option value="retry">Retry</option>
              <option value="notify">Notify</option>
            </select>
          </label>

          {onFailure.strategy === "retry" && (
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
                Max attempts
                <input
                  type="number"
                  min={1}
                  value={onFailure.maxAttempts ?? 3}
                  onChange={(e) => onChange({ ...onFailure, maxAttempts: Number(e.target.value) })}
                  aria-label="Max attempts"
                  style={{
                    padding: "var(--sy-space-1) var(--sy-space-2)",
                    borderRadius: "var(--sy-radius-sm)",
                    border: "1px solid var(--sy-color-line)",
                    background: "var(--sy-color-surface-1)",
                    color: "var(--sy-color-fg)",
                    fontSize: "0.875rem",
                    width: "6rem",
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
                }}
              >
                Backoff (seconds)
                <input
                  type="number"
                  min={0}
                  value={onFailure.backoffSeconds ?? 5}
                  onChange={(e) => onChange({ ...onFailure, backoffSeconds: Number(e.target.value) })}
                  aria-label="Backoff seconds"
                  style={{
                    padding: "var(--sy-space-1) var(--sy-space-2)",
                    borderRadius: "var(--sy-radius-sm)",
                    border: "1px solid var(--sy-color-line)",
                    background: "var(--sy-color-surface-1)",
                    color: "var(--sy-color-fg)",
                    fontSize: "0.875rem",
                    width: "6rem",
                  }}
                />
              </label>
            </>
          )}

          {onFailure.strategy === "notify" && (
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
                Notify entity
                <input
                  type="text"
                  value={onFailure.entity ?? ""}
                  onChange={(e) => onChange({ ...onFailure, entity: e.target.value })}
                  placeholder="notify.phone"
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
                }}
              >
                Message
                <textarea
                  value={onFailure.message ?? ""}
                  onChange={(e) => onChange({ ...onFailure, message: e.target.value })}
                  rows={2}
                  style={{
                    padding: "var(--sy-space-2)",
                    borderRadius: "var(--sy-radius-sm)",
                    border: "1px solid var(--sy-color-line)",
                    background: "var(--sy-color-surface-1)",
                    color: "var(--sy-color-fg)",
                    fontSize: "0.875rem",
                    resize: "vertical",
                  }}
                />
              </label>
            </>
          )}
        </div>
      )}
    </section>
  );
}
