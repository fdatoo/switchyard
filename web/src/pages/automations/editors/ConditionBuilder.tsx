/**
 * ConditionBuilder — recursive condition tree editor.
 *
 * Handles leaf types: StateEq, StateNeq, NumericGt, TimeInRange, Starlark
 * Group types: All (AndCondition), Any (OrCondition), Not (NotCondition)
 *
 * Starlark conditions render as locked (read-only with "View in Pkl editor →" link).
 */

import type { ConditionDraft } from "../useAutomationEditor";

interface ConditionBuilderProps {
  condition: ConditionDraft;
  onChange: (updated: ConditionDraft) => void;
  onRemove?: () => void;
  depth?: number;
}

export function ConditionBuilder({
  condition,
  onChange,
  onRemove,
  depth = 0,
}: ConditionBuilderProps) {
  const indent = depth * 16;

  if (condition.type === "Starlark") {
    return (
      <div
        style={{
          marginLeft: `${indent}px`,
          padding: "var(--sy-space-2) var(--sy-space-3)",
          background: "var(--sy-color-surface-2)",
          borderRadius: "var(--sy-radius-sm)",
          border: "1px solid var(--sy-color-line)",
          display: "flex",
          alignItems: "flex-start",
          gap: "var(--sy-space-2)",
        }}
      >
        <span
          style={{
            display: "inline-block",
            padding: "1px var(--sy-space-2)",
            borderRadius: "var(--sy-radius-pill)",
            background: "var(--sy-color-purple)",
            color: "var(--sy-color-bg)",
            fontSize: "0.6875rem",
            fontWeight: 600,
            flexShrink: 0,
          }}
        >
          starlark
        </span>
        <code
          style={{
            flex: 1,
            fontSize: "0.8125rem",
            color: "var(--sy-color-fg-3)",
            fontFamily: "var(--sy-font-mono, monospace)",
            whiteSpace: "pre-wrap",
            overflow: "hidden",
            display: "-webkit-box",
            WebkitLineClamp: 3,
            WebkitBoxOrient: "vertical",
          }}
        >
          {condition.starlarkExpr ?? ""}
        </code>
        {condition.starlarkFilePath && (
          <a
            href={`/_authed/pkl-editor/${encodeURIComponent(condition.starlarkFilePath)}${condition.starlarkLine ? `?line=${condition.starlarkLine}` : ""}`}
            style={{
              color: "var(--sy-color-accent)",
              fontSize: "0.8125rem",
              textDecoration: "none",
              whiteSpace: "nowrap",
              flexShrink: 0,
            }}
          >
            View in Pkl editor →
          </a>
        )}
      </div>
    );
  }

  if (condition.type === "All" || condition.type === "Any" || condition.type === "Not") {
    const label = condition.type === "All" ? "All of" : condition.type === "Any" ? "Any of" : "Not";
    const children = condition.children ?? [];

    const updateChild = (i: number, updated: ConditionDraft) => {
      const newChildren = [...children];
      newChildren[i] = updated;
      onChange({ ...condition, children: newChildren });
    };

    const removeChild = (i: number) => {
      const newChildren = children.filter((_, idx) => idx !== i);
      onChange({ ...condition, children: newChildren });
    };

    return (
      <div
        style={{
          marginLeft: `${indent}px`,
          padding: "var(--sy-space-2) var(--sy-space-3)",
          background: "var(--sy-color-surface-1)",
          borderRadius: "var(--sy-radius-sm)",
          border: "1px solid var(--sy-color-line)",
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            marginBottom: "var(--sy-space-2)",
          }}
        >
          <span
            style={{
              fontSize: "0.75rem",
              fontWeight: 600,
              textTransform: "uppercase",
              letterSpacing: "0.06em",
              color: "var(--sy-color-fg-4)",
            }}
          >
            {label}
          </span>
          {onRemove && (
            <button
              type="button"
              onClick={onRemove}
              style={{
                background: "none",
                border: "none",
                color: "var(--sy-color-fg-4)",
                cursor: "pointer",
                fontSize: "0.8125rem",
                padding: "0 var(--sy-space-1)",
              }}
              aria-label="Remove group"
            >
              × Remove
            </button>
          )}
        </div>
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            gap: "var(--sy-space-2)",
          }}
        >
          {children.map((child, i) => (
            <ConditionBuilder
              key={i}
              condition={child}
              onChange={(updated) => updateChild(i, updated)}
              onRemove={() => removeChild(i)}
              depth={depth + 1}
            />
          ))}
        </div>
      </div>
    );
  }

  // Leaf condition types
  return (
    <div
      style={{
        marginLeft: `${indent}px`,
        display: "flex",
        alignItems: "center",
        gap: "var(--sy-space-2)",
        padding: "var(--sy-space-2) var(--sy-space-3)",
        background: "var(--sy-color-surface-1)",
        borderRadius: "var(--sy-radius-sm)",
        border: "1px solid var(--sy-color-line)",
      }}
    >
      <select
        value={condition.type}
        onChange={(e) =>
          onChange({
            ...condition,
            type: e.target.value as ConditionDraft["type"],
          })
        }
        style={{
          padding: "var(--sy-space-1) var(--sy-space-2)",
          borderRadius: "var(--sy-radius-sm)",
          border: "1px solid var(--sy-color-line)",
          background: "var(--sy-color-surface-2)",
          color: "var(--sy-color-fg)",
          fontSize: "0.8125rem",
        }}
      >
        <option value="StateEq">State equals</option>
        <option value="StateNeq">State not equals</option>
        <option value="NumericGt">Numeric greater than</option>
        <option value="TimeInRange">Time in range</option>
      </select>

      {(condition.type === "StateEq" || condition.type === "StateNeq") && (
        <>
          <input
            type="text"
            placeholder="entity"
            value={condition.entity ?? ""}
            onChange={(e) => onChange({ ...condition, entity: e.target.value })}
            style={inputStyle}
            aria-label="Entity"
          />
          <input
            type="text"
            placeholder="value"
            value={condition.value ?? ""}
            onChange={(e) => onChange({ ...condition, value: e.target.value })}
            style={inputStyle}
            aria-label="Value"
          />
        </>
      )}

      {condition.type === "NumericGt" && (
        <>
          <input
            type="text"
            placeholder="entity"
            value={condition.entity ?? ""}
            onChange={(e) => onChange({ ...condition, entity: e.target.value })}
            style={inputStyle}
            aria-label="Entity"
          />
          <input
            type="number"
            placeholder="threshold"
            value={condition.numericValue ?? 0}
            onChange={(e) => onChange({ ...condition, numericValue: Number(e.target.value) })}
            style={{ ...inputStyle, width: "6rem" }}
            aria-label="Threshold"
          />
        </>
      )}

      {condition.type === "TimeInRange" && (
        <>
          <input
            type="time"
            value={condition.after ?? ""}
            onChange={(e) => onChange({ ...condition, after: e.target.value })}
            style={inputStyle}
            aria-label="After"
          />
          <span style={{ fontSize: "0.8125rem", color: "var(--sy-color-fg-4)" }}>to</span>
          <input
            type="time"
            value={condition.before ?? ""}
            onChange={(e) => onChange({ ...condition, before: e.target.value })}
            style={inputStyle}
            aria-label="Before"
          />
        </>
      )}

      <div style={{ flex: 1 }} />

      {onRemove && (
        <button
          type="button"
          onClick={onRemove}
          style={{
            background: "none",
            border: "none",
            color: "var(--sy-color-fg-4)",
            cursor: "pointer",
            fontSize: "0.8125rem",
            padding: "0 var(--sy-space-1)",
          }}
          aria-label="Remove condition"
        >
          × Remove
        </button>
      )}
    </div>
  );
}

const inputStyle: React.CSSProperties = {
  padding: "var(--sy-space-1) var(--sy-space-2)",
  borderRadius: "var(--sy-radius-sm)",
  border: "1px solid var(--sy-color-line)",
  background: "var(--sy-color-surface-1)",
  color: "var(--sy-color-fg)",
  fontSize: "0.8125rem",
};
