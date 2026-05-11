/**
 * DoSection — action list editor.
 *
 * Dispatches each action to its typed component by type + capability.
 * Starlark actions render as locked panels.
 * Sequence/Parallel blocks show nested actions (one level only in form).
 */

import { useState } from "react";
import type { ActionDraft } from "../useAutomationEditor";
import { TurnOnAction } from "../actions/TurnOnAction";
import { SetBrightnessAction } from "../actions/SetBrightnessAction";
import { RunScriptAction } from "../actions/RunScriptAction";
import { NotifyAction } from "../actions/NotifyAction";
import { CallCapabilityAction } from "../actions/CallCapabilityAction";
import { SceneAction } from "../actions/SceneAction";
import { WaitAction } from "../actions/WaitAction";
import { StarlarkActionLocked } from "../actions/StarlarkActionLocked";

interface DoSectionProps {
  actions: ActionDraft[];
  onChange: (actions: ActionDraft[]) => void;
}

function defaultAction(): ActionDraft {
  return { type: "TurnOn", entity: "" };
}

function ActionCard({
  action,
  onChange,
  onRemove,
}: {
  action: ActionDraft;
  onChange: (action: ActionDraft) => void;
  onRemove: () => void;
}) {
  if (action.type === "Starlark") {
    return <StarlarkActionLocked action={action} />;
  }

  const isBlock = action.type === "Sequence" || action.type === "Parallel";
  if (isBlock) {
    return (
      <div
        style={{
          padding: "var(--sy-space-2) var(--sy-space-3)",
          background: "var(--sy-color-surface-2)",
          borderRadius: "var(--sy-radius-sm)",
          border: "1px solid var(--sy-color-line)",
          display: "flex",
          flexDirection: "column",
          gap: "var(--sy-space-2)",
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            fontSize: "0.8125rem",
            color: "var(--sy-color-fg-3)",
          }}
        >
          <span>{action.type} block ({(action.children ?? []).length} actions)</span>
          <a
            href="/_authed/pkl-editor"
            style={{ color: "var(--sy-color-accent)", textDecoration: "none", fontSize: "0.8125rem" }}
          >
            Open in Pkl editor →
          </a>
        </div>
        <button
          type="button"
          onClick={onRemove}
          style={{
            alignSelf: "flex-end",
            background: "none",
            border: "none",
            color: "var(--sy-color-fg-4)",
            cursor: "pointer",
            fontSize: "0.8125rem",
            padding: "0",
          }}
        >
          × Remove
        </button>
      </div>
    );
  }

  return (
    <div
      style={{
        padding: "var(--sy-space-2) var(--sy-space-3)",
        background: "var(--sy-color-surface-1)",
        borderRadius: "var(--sy-radius-sm)",
        border: "1px solid var(--sy-color-line)",
        display: "flex",
        flexDirection: "column",
        gap: "var(--sy-space-2)",
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <select
          value={action.type}
          onChange={(e) =>
            onChange({ type: e.target.value as ActionDraft["type"] })
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
          <option value="TurnOn">Turn on</option>
          <option value="SetBrightness">Set brightness</option>
          <option value="Notify">Notify</option>
          <option value="RunScript">Run script</option>
          <option value="Scene">Activate scene</option>
          <option value="Wait">Wait</option>
          <option value="CallCapability">Call capability</option>
        </select>

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
          aria-label="Remove action"
        >
          × Remove
        </button>
      </div>

      {action.type === "TurnOn" && <TurnOnAction action={action} onChange={onChange} />}
      {action.type === "SetBrightness" && <SetBrightnessAction action={action} onChange={onChange} />}
      {action.type === "Notify" && <NotifyAction action={action} onChange={onChange} />}
      {action.type === "RunScript" && <RunScriptAction action={action} onChange={onChange} />}
      {action.type === "Scene" && <SceneAction action={action} onChange={onChange} />}
      {action.type === "Wait" && <WaitAction action={action} onChange={onChange} />}
      {action.type === "CallCapability" && <CallCapabilityAction action={action} onChange={onChange} />}
    </div>
  );
}

export function DoSection({ actions, onChange }: DoSectionProps) {
  const [collapsed, setCollapsed] = useState(false);

  const summary =
    actions.length === 0
      ? "No actions"
      : `${actions.length} action${actions.length !== 1 ? "s" : ""}`;

  function addAction() {
    onChange([...actions, defaultAction()]);
  }

  function updateAction(i: number, updated: ActionDraft) {
    const next = [...actions];
    next[i] = updated;
    onChange(next);
  }

  function removeAction(i: number) {
    onChange(actions.filter((_, idx) => idx !== i));
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
        <span>Do</span>
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
          {actions.length === 0 ? (
            <p
              style={{
                margin: 0,
                fontSize: "0.8125rem",
                color: "var(--sy-color-fg-4)",
                fontStyle: "italic",
              }}
            >
              No actions defined yet.
            </p>
          ) : (
            actions.map((a, i) => (
              <ActionCard
                key={i}
                action={a}
                onChange={(updated) => updateAction(i, updated)}
                onRemove={() => removeAction(i)}
              />
            ))
          )}

          <button
            type="button"
            onClick={addAction}
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
            + Add action
          </button>
        </div>
      )}
    </section>
  );
}
