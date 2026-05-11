/**
 * useAutomationEditor — state machine for the automation editor.
 *
 * Wraps Plan 11's useEditSession with automation-specific:
 *   - editorState: parsed AutomationEditorState from astJson
 *   - pklSource: live Pkl preview bytes (from RegenPreview or last committed)
 *   - isDirty: true after any update
 *   - Updater functions for each section
 *   - save() / discard() delegated to the edit session
 *   - conflict state surfaced from the edit session
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { editSessionClient } from "@/edit-session/client";
import { useEditSession } from "@/edit-session/useEditSession";
import type { ConflictState } from "@/edit-session/useEditSession";

// ---------------------------------------------------------------------------
// Domain types for the editor state
// ---------------------------------------------------------------------------

export type TriggerType = "SunEvent" | "Time" | "EntityStateChange" | "Webhook" | "Manual";

export interface TriggerDraft {
  type: TriggerType;
  // SunEvent
  sunEvent?: "sunrise" | "sunset";
  offsetMinutes?: number;
  // Time
  timeAt?: string;
  cron?: string;
  useCron?: boolean;
  // EntityStateChange
  entities?: string[];
  from?: string;
  to?: string;
  forDurNs?: number;
  // Webhook
  path?: string;
  methods?: string[];
}

export interface ConditionDraft {
  type: "StateEq" | "StateNeq" | "NumericGt" | "TimeInRange" | "All" | "Any" | "Not" | "Starlark";
  entity?: string;
  value?: string;
  numericValue?: number;
  after?: string;
  before?: string;
  children?: ConditionDraft[];
  starlarkExpr?: string;
  starlarkFilePath?: string;
  starlarkLine?: number;
}

export type ActionType =
  | "TurnOn"
  | "SetBrightness"
  | "RunScript"
  | "Notify"
  | "CallCapability"
  | "Scene"
  | "Wait"
  | "Starlark"
  | "Sequence"
  | "Parallel";

export interface ActionDraft {
  type: ActionType;
  entity?: string;
  capability?: string;
  brightness?: number;
  scriptName?: string;
  message?: string;
  sceneName?: string;
  durationValue?: number;
  durationUnit?: "s" | "min" | "h";
  starlarkBody?: string;
  starlarkFilePath?: string;
  starlarkLine?: number;
  children?: ActionDraft[];
  args?: Record<string, string>;
}

export type OnFailureStrategyType = "ignore" | "retry" | "notify";

export interface OnFailureDraft {
  strategy: OnFailureStrategyType;
  maxAttempts?: number;
  backoffSeconds?: number;
  entity?: string;
  message?: string;
}

export interface AutomationEditorState {
  id: string;
  displayName?: string;
  enabled: boolean;
  trigger: TriggerDraft | null;
  multipleTriggersDetected: boolean;
  conditions: ConditionDraft[];
  actions: ActionDraft[];
  onFailure: OnFailureDraft;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function defaultEditorState(id: string): AutomationEditorState {
  return {
    id,
    enabled: true,
    trigger: null,
    multipleTriggersDetected: false,
    conditions: [],
    actions: [],
    onFailure: { strategy: "ignore" },
  };
}

/** Parse astJson from the edit session into editor state. */
function parseAstJson(astJson: string | null, id: string): AutomationEditorState {
  if (!astJson) return defaultEditorState(id);
  try {
    const raw = JSON.parse(astJson) as Record<string, unknown>;
    const state = defaultEditorState(String(raw.id ?? id));
    state.enabled = Boolean(raw.enabled ?? true);

    // Parse triggers
    const triggers = (raw.triggers as unknown[]) ?? [];
    if (triggers.length > 1) {
      state.multipleTriggersDetected = true;
      state.trigger = null;
    } else if (triggers.length === 1) {
      state.trigger = parseTrigger(triggers[0] as Record<string, unknown>);
    }

    // Parse conditions
    const conditions = (raw.conditions as unknown[]) ?? [];
    state.conditions = conditions.map((c) => parseCondition(c as Record<string, unknown>));

    // Parse actions
    const actions = (raw.actions as unknown[]) ?? [];
    state.actions = actions.map((a) => parseAction(a as Record<string, unknown>));

    return state;
  } catch {
    return defaultEditorState(id);
  }
}

function parseTrigger(raw: Record<string, unknown>): TriggerDraft {
  if (raw.time) {
    const t = raw.time as Record<string, unknown>;
    return { type: "Time", timeAt: String(t.at ?? ""), cron: String(t.cron ?? "") };
  }
  if (raw.event) {
    const ev = raw.event as Record<string, unknown>;
    const kind = String(ev.kind ?? "");
    if (kind === "sun.sunrise") return { type: "SunEvent", sunEvent: "sunrise" };
    if (kind === "sun.sunset") return { type: "SunEvent", sunEvent: "sunset" };
    if (kind === "switchyard.manual") return { type: "Manual" };
    return { type: "Manual" };
  }
  if (raw.stateChange) {
    const sc = raw.stateChange as Record<string, unknown>;
    return {
      type: "EntityStateChange",
      entities: (sc.entities as string[]) ?? [],
      from: String(sc.from ?? ""),
      to: String(sc.to ?? ""),
    };
  }
  if (raw.webhook) {
    const wh = raw.webhook as Record<string, unknown>;
    return {
      type: "Webhook",
      path: String(wh.path ?? ""),
      methods: (wh.methods as string[]) ?? ["POST"],
    };
  }
  return { type: "Manual" };
}

function parseCondition(raw: Record<string, unknown>): ConditionDraft {
  if (raw.starlark) {
    return { type: "Starlark", starlarkExpr: String((raw.starlark as Record<string, unknown>).expr ?? "") };
  }
  if (raw.state) {
    const s = raw.state as Record<string, unknown>;
    if (s.not) return { type: "StateNeq", entity: String(s.entity ?? ""), value: String(s.not) };
    return { type: "StateEq", entity: String(s.entity ?? ""), value: String(s.equals ?? "") };
  }
  if (raw.numeric) {
    const n = raw.numeric as Record<string, unknown>;
    return { type: "NumericGt", entity: String(n.entity ?? ""), numericValue: Number(n.value ?? 0) };
  }
  if (raw.time) {
    const t = raw.time as Record<string, unknown>;
    return { type: "TimeInRange", after: String(t.after ?? ""), before: String(t.before ?? "") };
  }
  if (raw.and) {
    const a = raw.and as Record<string, unknown>;
    return { type: "All", children: ((a.all as unknown[]) ?? []).map((c) => parseCondition(c as Record<string, unknown>)) };
  }
  if (raw.or) {
    const o = raw.or as Record<string, unknown>;
    return { type: "Any", children: ((o.any as unknown[]) ?? []).map((c) => parseCondition(c as Record<string, unknown>)) };
  }
  return { type: "StateEq", entity: "", value: "" };
}

function parseAction(raw: Record<string, unknown>): ActionDraft {
  if (raw.starlark) {
    return { type: "Starlark", starlarkBody: String((raw.starlark as Record<string, unknown>).body ?? "") };
  }
  if (raw.callService) {
    const cs = raw.callService as Record<string, unknown>;
    const cap = String(cs.capability ?? "");
    if (cap === "turn_on") return { type: "TurnOn", entity: String(cs.entity ?? "") };
    if (cap === "set_brightness") {
      const args = (cs.args as Record<string, string>) ?? {};
      return { type: "SetBrightness", entity: String(cs.entity ?? ""), brightness: Number(args.level ?? 50) };
    }
    if (cap === "notify") return { type: "Notify", entity: String(cs.entity ?? ""), message: String(((cs.args as Record<string, string>) ?? {}).message ?? "") };
    return { type: "CallCapability", entity: String(cs.entity ?? ""), capability: cap };
  }
  if (raw.scene) {
    return { type: "Scene", sceneName: String((raw.scene as Record<string, unknown>).slug ?? "") };
  }
  if (raw.script) {
    return { type: "RunScript", scriptName: String((raw.script as Record<string, unknown>).name ?? "") };
  }
  if (raw.wait) {
    const ns = Number((raw.wait as Record<string, unknown>).durationNs ?? 0);
    return { type: "Wait", durationValue: Math.round(ns / 1e9), durationUnit: "s" };
  }
  if (raw.sequence) {
    const s = raw.sequence as Record<string, unknown>;
    return { type: "Sequence", children: ((s.actions as unknown[]) ?? []).map((a) => parseAction(a as Record<string, unknown>)) };
  }
  if (raw.parallel) {
    const p = raw.parallel as Record<string, unknown>;
    return { type: "Parallel", children: ((p.actions as unknown[]) ?? []).map((a) => parseAction(a as Record<string, unknown>)) };
  }
  return { type: "CallCapability", entity: "", capability: "" };
}

// ---------------------------------------------------------------------------
// Hook return types
// ---------------------------------------------------------------------------

export interface UseAutomationEditorReturn {
  editorState: AutomationEditorState;
  pklSource: string;
  isDirty: boolean;
  conflict: ConflictState | null;
  sessionStatus: string;
  sessionError: string | null;
  updateTrigger: (trigger: TriggerDraft | null) => void;
  updateConditions: (conditions: ConditionDraft[]) => void;
  updateActions: (actions: ActionDraft[]) => void;
  updateOnFailure: (onFailure: OnFailureDraft) => void;
  save: () => Promise<void>;
  discard: () => Promise<void>;
  resolveForce: () => Promise<void>;
  resolveOpenMerge: () => void;
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useAutomationEditor(
  slug: string,
  filePath: string,
): UseAutomationEditorReturn {
  const session = useEditSession(filePath, editSessionClient);

  // Local editor state derived from / independent of the session
  const [editorState, setEditorState] = useState<AutomationEditorState>(() =>
    defaultEditorState(slug),
  );
  const [pklSource, setPklSource] = useState<string>("");
  const [isDirty, setIsDirty] = useState(false);

  // Sync editorState from astJson when the session opens
  const lastAstRef = useRef<string | null>(null);
  useEffect(() => {
    if (session.astJson && session.astJson !== lastAstRef.current) {
      lastAstRef.current = session.astJson;
      setEditorState(parseAstJson(session.astJson, slug));
      // Seed pklSource from the ancestor pkl
      setPklSource(session.astJson);
      setIsDirty(false);
    }
  }, [session.astJson, slug]);

  const updateTrigger = useCallback((trigger: TriggerDraft | null) => {
    setEditorState((prev) => ({ ...prev, trigger }));
    setIsDirty(true);
  }, []);

  const updateConditions = useCallback((conditions: ConditionDraft[]) => {
    setEditorState((prev) => ({ ...prev, conditions }));
    setIsDirty(true);
  }, []);

  const updateActions = useCallback((actions: ActionDraft[]) => {
    setEditorState((prev) => ({ ...prev, actions }));
    setIsDirty(true);
  }, []);

  const updateOnFailure = useCallback((onFailure: OnFailureDraft) => {
    setEditorState((prev) => ({ ...prev, onFailure }));
    setIsDirty(true);
  }, []);

  const save = useCallback(async () => {
    await session.save(pklSource);
    setIsDirty(false);
  }, [session, pklSource]);

  const discard = useCallback(async () => {
    await session.discard();
    setIsDirty(false);
  }, [session]);

  const resolveForce = useCallback(async () => {
    await session.resolveConflict({ kind: "force", stagedPkl: pklSource }, pklSource);
    setIsDirty(false);
  }, [session, pklSource]);

  const resolveOpenMerge = useCallback(() => {
    const encoded = encodeURIComponent(filePath);
    window.location.href = `/_authed/pkl-editor?file=${encoded}&merge=true`;
  }, [filePath]);

  return {
    editorState,
    pklSource,
    isDirty,
    conflict: session.conflict,
    sessionStatus: session.status,
    sessionError: session.error,
    updateTrigger,
    updateConditions,
    updateActions,
    updateOnFailure,
    save,
    discard,
    resolveForce,
    resolveOpenMerge,
  };
}
