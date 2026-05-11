/**
 * WhenSection — condition tree editor.
 * Renders ConditionBuilder for each condition, with add/empty affordances.
 */

import { useState } from "react";
import type { ConditionDraft } from "../useAutomationEditor";
import { ConditionBuilder } from "../editors/ConditionBuilder";

interface WhenSectionProps {
  conditions: ConditionDraft[];
  onChange: (conditions: ConditionDraft[]) => void;
}

function defaultCondition(): ConditionDraft {
  return { type: "StateEq", entity: "", value: "" };
}

export function WhenSection({ conditions, onChange }: WhenSectionProps) {
  const [collapsed, setCollapsed] = useState(false);

  const summary =
    conditions.length === 0
      ? "No conditions"
      : `${conditions.length} condition${conditions.length !== 1 ? "s" : ""}`;

  function addCondition() {
    onChange([...conditions, defaultCondition()]);
  }

  function updateCondition(i: number, updated: ConditionDraft) {
    const next = [...conditions];
    next[i] = updated;
    onChange(next);
  }

  function removeCondition(i: number) {
    onChange(conditions.filter((_, idx) => idx !== i));
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
        <span>When</span>
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
            gap: "var(--sy-space-2)",
          }}
        >
          {conditions.length === 0 ? (
            <p
              style={{
                margin: 0,
                fontSize: "0.8125rem",
                color: "var(--sy-color-fg-4)",
                fontStyle: "italic",
              }}
            >
              No conditions — this automation runs unconditionally.
            </p>
          ) : (
            conditions.map((c, i) => (
              <ConditionBuilder
                key={i}
                condition={c}
                onChange={(updated) => updateCondition(i, updated)}
                onRemove={() => removeCondition(i)}
              />
            ))
          )}

          <button
            type="button"
            onClick={addCondition}
            style={{
              alignSelf: "flex-start",
              padding: "var(--sy-space-1) var(--sy-space-3)",
              borderRadius: "var(--sy-radius-sm)",
              border: "1px dashed var(--sy-color-line)",
              background: "none",
              color: "var(--sy-color-fg-3)",
              cursor: "pointer",
              fontSize: "0.8125rem",
              marginTop: "var(--sy-space-1)",
            }}
          >
            + Add condition
          </button>
        </div>
      )}
    </section>
  );
}
