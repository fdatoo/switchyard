/**
 * AutomationEditor — full editor for a single automation.
 *
 * Wires four section cards (Trigger, When, Do, OnFailure) with the
 * useAutomationEditor hook (Plan 10's edit state machine).
 *
 * Plan 11's edit session infrastructure provides save/discard/conflict flows.
 * Plan 11's ConflictBanner is shown when an external edit conflict is detected.
 *
 * Layout: grid with 1fr sections + 380px PklSourcePane.
 * Below 760px: PklSourcePane is hidden; "View Pkl source" button shows in header.
 */

import { useState } from "react";
import { useAutomationEditor } from "./useAutomationEditor";
import { TriggerSection } from "./sections/TriggerSection";
import { WhenSection } from "./sections/WhenSection";
import { DoSection } from "./sections/DoSection";
import { OnFailureSection } from "./sections/OnFailureSection";
import { PklSourcePane } from "./PklSourcePane";
import { ConflictBanner } from "@/edit-session/conflict-ui";
import { Button } from "@/theme/primitives/button";

interface AutomationEditorProps {
  slug: string;
}

export function AutomationEditor({ slug }: AutomationEditorProps) {
  const filePath = `automations/${slug}.pkl`;
  const {
    editorState,
    pklSource,
    isDirty,
    conflict,
    sessionError,
    updateTrigger,
    updateConditions,
    updateActions,
    updateOnFailure,
    save,
    discard,
    resolveForce,
    resolveOpenMerge,
  } = useAutomationEditor(slug, filePath);

  const [showPklSource, setShowPklSource] = useState(false);
  const [isSaving, setIsSaving] = useState(false);

  async function handleSave() {
    setIsSaving(true);
    try {
      await save();
      // Navigate to list on success
      window.location.href = "/_authed/automations";
    } catch {
      // Error handled by hook (sets conflict or sessionError)
    } finally {
      setIsSaving(false);
    }
  }

  async function handleDiscard() {
    await discard();
    window.location.href = "/_authed/automations";
  }

  function handleRunNow() {
    // Call AutomationService.Trigger and navigate to time-machine
    // TODO(plan-10 wire): replace with real RPC
    const runId = `run-${Date.now()}`;
    window.location.href = `/_authed/time-machine/${runId}`;
  }

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        height: "100%",
      }}
    >
      {/* Header */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "var(--sy-space-3)",
          padding: "var(--sy-space-4) var(--sy-space-6)",
          borderBottom: "1px solid var(--sy-color-line)",
          flexShrink: 0,
        }}
      >
        <a
          href="/_authed/automations"
          style={{
            color: "var(--sy-color-fg-4)",
            textDecoration: "none",
            fontSize: "0.8125rem",
          }}
        >
          ← Automations
        </a>
        <h1
          style={{
            margin: 0,
            fontSize: "1.125rem",
            fontWeight: 600,
            color: "var(--sy-color-fg)",
            flex: 1,
          }}
        >
          {editorState.displayName || slug}
        </h1>

        {/* Mobile: View Pkl source toggle */}
        <button
          type="button"
          onClick={() => setShowPklSource((v) => !v)}
          className="pkl-source-toggle"
          style={{
            display: "none", // shown via media query
            padding: "var(--sy-space-1) var(--sy-space-3)",
            borderRadius: "var(--sy-radius-sm)",
            border: "1px solid var(--sy-color-line)",
            background: "transparent",
            color: "var(--sy-color-fg-3)",
            cursor: "pointer",
            fontSize: "0.8125rem",
          }}
          aria-pressed={showPklSource}
        >
          {showPklSource ? "Hide Pkl source" : "View Pkl source"}
        </button>

        <Button
          type="button"
          variant="ghost"
          onClick={() => void handleRunNow()}
          style={{ fontSize: "0.875rem" }}
        >
          Run now
        </Button>

        <Button
          type="button"
          variant="secondary"
          onClick={() => void handleDiscard()}
          style={{ fontSize: "0.875rem" }}
        >
          Discard
        </Button>

        <Button
          type="button"
          variant="primary"
          onClick={() => void handleSave()}
          disabled={!isDirty || isSaving}
          style={{
            fontSize: "0.875rem",
            opacity: !isDirty || isSaving ? 0.5 : 1,
          }}
        >
          Save &amp; exit
        </Button>
      </div>

      {/* Conflict banner */}
      {conflict && (
        <div style={{ padding: "var(--sy-space-3) var(--sy-space-6)" }}>
          <ConflictBanner
            filePath={filePath}
            dirtyCount={isDirty ? 1 : 0}
            onDiscard={() => void handleDiscard()}
            onForceOverwrite={() => void resolveForce()}
            onOpenMerge={resolveOpenMerge}
          />
        </div>
      )}

      {/* Session error */}
      {sessionError && (
        <p
          style={{
            margin: "var(--sy-space-2) var(--sy-space-6)",
            fontSize: "0.8125rem",
            color: "var(--sy-color-bad)",
          }}
        >
          Error: {sessionError}
        </p>
      )}

      {/* Main grid: sections + source pane */}
      <div
        className="automation-editor-grid"
        style={{
          display: "grid",
          gridTemplateColumns: "1fr 380px",
          gap: "var(--sy-space-4)",
          padding: "var(--sy-space-4) var(--sy-space-6)",
          flex: 1,
          overflow: "auto",
          alignItems: "start",
        }}
      >
        {/* Left: four section cards */}
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            gap: "var(--sy-space-3)",
          }}
        >
          <TriggerSection
            trigger={editorState.trigger}
            onChange={updateTrigger}
            multipleTriggersDetected={editorState.multipleTriggersDetected}
          />

          <WhenSection
            conditions={editorState.conditions}
            onChange={updateConditions}
          />

          <DoSection
            actions={editorState.actions}
            onChange={updateActions}
          />

          <OnFailureSection
            onFailure={editorState.onFailure}
            onChange={updateOnFailure}
          />
        </div>

        {/* Right: Pkl source pane (hidden on mobile) */}
        <div
          className="pkl-source-pane-wrapper"
          style={{
            position: "sticky",
            top: 0,
          }}
        >
          <PklSourcePane source={pklSource} isDirty={isDirty} />
        </div>
      </div>
    </div>
  );
}
