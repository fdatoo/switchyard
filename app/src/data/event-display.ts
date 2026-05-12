/**
 * Event presentation helpers — map an EventRecord to the props
 * SyEventRow expects (icon, intent, title, meta).
 *
 * Why this lives separately from data/activity.ts: the wire shape
 * (kind/entity/payload_json) is stable, but how we render it is a
 * product decision that may change (different icons, different intent
 * mapping, richer titles). Keeping the rendering rules in one place
 * means the activity views, the driver panel's recent-events list, and
 * any future event surface all stay in lockstep.
 */

import type { EventRecord, InterestingnessTag } from "./activity";
import type { IconName } from "@/lib/components/icon/SyIcon.vue";
import type { EventRowTag } from "@/lib/components/event-row/SyEventRow.vue";

type Intent = "neutral" | "accent" | "good" | "warn" | "bad" | "info" | "automation";

export interface EventPresentation {
  icon: IconName;
  intent: Intent;
  title: string;
  meta: string;
  timestamp: string;
  tags: EventRowTag[];
}

/** Map a tag category to a SyBadge intent. Shared by the row stripe and
    the detail rail's full tag list so they stay color-consistent. */
export function tagBadgeIntent(t: InterestingnessTag): EventRowTag["intent"] {
  switch (t.category) {
    case "failure":       return "bad";
    case "security":      return "bad";
    case "performance":   return "warn";
    case "anomaly":       return "warn";
    case "causation":     return "purple";
    case "novelty":       return "accent";
    case "configuration": return "info";
  }
  return "neutral";
}

interface KindRule {
  match: (kind: string) => boolean;
  icon: IconName;
  intent: Intent;
  /** Human-readable verb phrase; entity is appended by the caller. */
  verb: string;
}

/* Order matters — first match wins. More specific rules come before
   broader fallbacks. */
/* Kinds on the wire are short strings (the daemon stores them as the
   eventstore's `kind` column — see proto/switchyard/event/v1/event.proto).
   We accept both the short form ("config", "system") and the
   payload-field form ("config_applied", "configApplied") so the table
   keeps working if the daemon's naming ever drifts. */
const RULES: KindRule[] = [
  { match: (k) => k === "state_changed" || k === "state",          icon: "bulb",     intent: "info",       verb: "state changed" },
  { match: (k) => k === "command_issued" || k === "command",       icon: "power",    intent: "accent",     verb: "command issued" },
  { match: (k) => k === "command_ack",                             icon: "check",    intent: "good",       verb: "ack" },
  { match: (k) => k === "automation_triggered" || k === "automation_run", icon: "sparkle",  intent: "automation", verb: "automation triggered" },
  { match: (k) => k === "automation_finished",                     icon: "sparkle",  intent: "automation", verb: "automation finished" },
  { match: (k) => k === "script_invoked" || k === "script",        icon: "sparkle",  intent: "automation", verb: "script invoked" },
  { match: (k) => k === "script_finished",                         icon: "sparkle",  intent: "automation", verb: "script finished" },
  { match: (k) => k === "entity_registered",                       icon: "plus",     intent: "good",       verb: "entity registered" },
  { match: (k) => k === "entity_unregistered",                     icon: "close",    intent: "warn",       verb: "entity unregistered" },
  { match: (k) => k === "driver_event",                            icon: "plugin",   intent: "info",       verb: "driver event" },
  { match: (k) => k === "driver_instance_restarted",               icon: "plugin",   intent: "warn",       verb: "driver restarted" },
  { match: (k) => k === "config" || k === "config_applied",        icon: "settings", intent: "accent",     verb: "config applied" },
  { match: (k) => k === "config_file_edited",                      icon: "settings", intent: "info",       verb: "config file edited" },
  { match: (k) => k === "webhook_received",                        icon: "activity", intent: "info",       verb: "webhook received" },
  { match: (k) => k === "auth_event" || k === "auth",              icon: "alert",    intent: "warn",       verb: "auth event" },
  { match: (k) => k === "system",                                  icon: "settings", intent: "neutral",    verb: "system event" },
];

const FALLBACK: Omit<KindRule, "match"> = {
  icon: "activity", intent: "neutral", verb: "event",
};

function ruleFor(kind: string): Omit<KindRule, "match"> {
  for (const r of RULES) if (r.match(kind)) return r;
  return FALLBACK;
}

/* Tier breakpoints for the relative timestamp. We don't use Intl
   RelativeTimeFormat directly because we want a tight, locale-neutral
   display that fits inside the row's trailing column. */
const REL_BREAKS: { ms: number; format: (delta: number) => string }[] = [
  { ms: 60_000,       format: (d) => `${Math.max(1, Math.floor(d / 1000))}s` },
  { ms: 60 * 60_000,  format: (d) => `${Math.floor(d / 60_000)}m` },
  { ms: 24 * 3.6e6,   format: (d) => `${Math.floor(d / 3.6e6)}h` },
  { ms: 7 * 24 * 3.6e6, format: (d) => `${Math.floor(d / (24 * 3.6e6))}d` },
];

/**
 * Format a timestamp as a compact "Nm ago" string for recent events,
 * falling back to a localized date for older events. The threshold is
 * one week; beyond that, exact-time becomes more useful than relative.
 */
export function formatEventTimestamp(occurredAt: Date, now: Date = new Date()): string {
  const delta = now.getTime() - occurredAt.getTime();
  if (delta < 0) return "now";
  for (const b of REL_BREAKS) {
    if (delta < b.ms) return `${b.format(delta)} ago`;
  }
  return occurredAt.toLocaleDateString(undefined, {
    month: "short", day: "numeric",
  });
}

/**
 * Build a one-line title for an event. For state-change / command /
 * driver / config / automation events we parse `payload_json` to
 * produce a verb-driven phrase ("light.kitchen → on", "config applied:
 * +2 drivers"). Everything else falls back to the kind's generic verb.
 *
 * Payload parsing is best-effort and locally typed — we tolerate
 * unexpected shapes by silently dropping back to the generic phrasing.
 */
export function presentEvent(e: EventRecord, now: Date = new Date()): EventPresentation {
  const rule = ruleFor(e.kind);
  const payload = safeParseJson(e.payloadJson);
  const title = titleFromPayload(e, rule.verb, payload);
  const meta = e.source || (e.causationId ? `caused by ${e.causationId.slice(0, 8)}…` : "");
  /* Compress tags into category-only badges; the full category+name lives
     in the detail rail where horizontal space allows it. */
  const tags: EventRowTag[] = e.tags.map((t) => ({
    intent: tagBadgeIntent(t),
    label: t.category,
  }));
  return {
    icon: rule.icon,
    intent: rule.intent,
    title,
    meta,
    timestamp: formatEventTimestamp(e.occurredAt, now),
    tags,
  };
}

/* ---- Payload parsing ------------------------------------------------ */

function safeParseJson(raw: string): unknown {
  if (!raw) return null;
  try { return JSON.parse(raw); } catch { return null; }
}

/** Type-narrow helper: look up a string property without unsafe casts. */
function pickStr(o: unknown, key: string): string | undefined {
  if (typeof o !== "object" || o === null) return undefined;
  const v = (o as Record<string, unknown>)[key];
  return typeof v === "string" ? v : undefined;
}
function pickObj(o: unknown, key: string): unknown {
  if (typeof o !== "object" || o === null) return undefined;
  return (o as Record<string, unknown>)[key];
}
function pickNum(o: unknown, key: string): number | undefined {
  if (typeof o !== "object" || o === null) return undefined;
  const v = (o as Record<string, unknown>)[key];
  return typeof v === "number" ? v : undefined;
}
function pickBool(o: unknown, key: string): boolean | undefined {
  if (typeof o !== "object" || o === null) return undefined;
  const v = (o as Record<string, unknown>)[key];
  return typeof v === "boolean" ? v : undefined;
}

/**
 * Try to produce a richer human title from the payload for a few high-
 * volume kinds. Returns `${entity} ${fallbackVerb}` when no specific
 * formatter applies so generic events still read sensibly.
 */
function titleFromPayload(e: EventRecord, fallbackVerb: string, payload: unknown): string {
  const ent = e.entity;
  /* Normalize the daemon's short kind names to the proto's variant names
     so the switch reads as one canonical form. */
  const kind = (() => {
    switch (e.kind) {
      case "config":         return "config_applied";
      case "state":          return "state_changed";
      case "command":        return "command_issued";
      case "automation_run": return "automation_triggered";
      case "script":         return "script_invoked";
      case "auth":           return "auth_event";
    }
    return e.kind;
  })();
  switch (kind) {
    case "state_changed": {
      const sc = pickObj(payload, "stateChanged") ?? pickObj(payload, "state_changed");
      const attrs = pickObj(sc, "attributes");
      const on = pickBool(attrs, "on") ?? pickBool(attrs, "is_on");
      const bri = pickNum(attrs, "brightness");
      if (on !== undefined) {
        const briPart = bri !== undefined ? ` (${Math.round(bri * 100)}%)` : "";
        return `${ent} → ${on ? "on" : "off"}${briPart}`;
      }
      return `${ent} state changed`;
    }
    case "command_issued": {
      const ci = pickObj(payload, "commandIssued") ?? pickObj(payload, "command_issued");
      const cmd = pickStr(ci, "command");
      if (cmd) return `${ent} ← ${cmd}`;
      return `${ent} command issued`;
    }
    case "command_ack": {
      const ca = pickObj(payload, "commandAck") ?? pickObj(payload, "command_ack");
      const ok = pickBool(ca, "success");
      if (ok === true)  return `${ent} ack`;
      if (ok === false) return `${ent} ack failed`;
      return `${ent} command ack`;
    }
    case "driver_event": {
      const de = pickObj(payload, "driverEvent") ?? pickObj(payload, "driver_event");
      const kind = pickStr(de, "kind");
      const detail = pickStr(de, "detail");
      const id = pickStr(de, "driver_instance_id") ?? pickStr(de, "driverInstanceId") ?? "";
      const head = id ? `${id} ${kind ?? "event"}` : `driver ${kind ?? "event"}`;
      return detail ? `${head}: ${detail}` : head;
    }
    case "driver_instance_restarted": {
      const dir = pickObj(payload, "driverInstanceRestarted") ?? pickObj(payload, "driver_instance_restarted");
      const id = pickStr(dir, "driver_instance_id") ?? pickStr(dir, "driverInstanceId") ?? "";
      const reason = pickStr(dir, "reason");
      const head = id ? `${id} restarted` : "driver restarted";
      return reason ? `${head} (${reason})` : head;
    }
    case "config_applied": {
      const ca = pickObj(payload, "configApplied") ?? pickObj(payload, "config_applied");
      const added   = pickNum(ca, "driver_instances_added")   ?? pickNum(ca, "driverInstancesAdded")   ?? 0;
      const removed = pickNum(ca, "driver_instances_removed") ?? pickNum(ca, "driverInstancesRemoved") ?? 0;
      const changed = pickNum(ca, "driver_instances_changed") ?? pickNum(ca, "driverInstancesChanged") ?? 0;
      const automations = pickNum(ca, "automations_changed") ?? pickNum(ca, "automationsChanged") ?? 0;
      const parts: string[] = [];
      if (added)       parts.push(`+${added} driver`);
      if (removed)     parts.push(`−${removed} driver`);
      if (changed)     parts.push(`Δ${changed} driver`);
      if (automations) parts.push(`Δ${automations} automation`);
      const dryRun = pickBool(ca, "dry_run") ?? pickBool(ca, "dryRun");
      const tail = dryRun ? " (dry run)" : "";
      return parts.length ? `config applied: ${parts.join(", ")}${tail}` : `config applied${tail}`;
    }
    case "automation_triggered": {
      const at = pickObj(payload, "automationTriggered") ?? pickObj(payload, "automation_triggered");
      const id = pickStr(at, "automation_id") ?? pickStr(at, "automationId") ?? "automation";
      const why = pickStr(at, "trigger_kind") ?? pickStr(at, "triggerKind");
      return why ? `${id} triggered by ${why}` : `${id} triggered`;
    }
    case "automation_finished": {
      const af = pickObj(payload, "automationFinished") ?? pickObj(payload, "automation_finished");
      const id = pickStr(af, "automation_id") ?? pickStr(af, "automationId") ?? "automation";
      const outcome = outcomeLabel(af);
      return `${id} ${outcome}`;
    }
    case "entity_registered": {
      const er = pickObj(payload, "entityRegistered") ?? pickObj(payload, "entity_registered");
      const name = pickStr(er, "friendly_name") ?? pickStr(er, "friendlyName");
      return name ? `${ent} registered (${name})` : `${ent} registered`;
    }
    case "entity_unregistered": {
      const eu = pickObj(payload, "entityUnregistered") ?? pickObj(payload, "entity_unregistered");
      const reason = pickStr(eu, "reason");
      return reason ? `${ent} unregistered (${reason})` : `${ent} unregistered`;
    }
    case "system": {
      const sys = pickObj(payload, "system");
      const kind = pickStr(sys, "kind");
      return kind ? `system: ${kind}` : "system event";
    }
  }
  return ent ? `${ent} ${fallbackVerb}` : fallbackVerb;
}

/**
 * Translate the AutomationFinished `outcome` value (string or numeric
 * enum, depending on which JSON encoder the daemon uses) into a verb.
 */
function outcomeLabel(af: unknown): string {
  if (typeof af !== "object" || af === null) return "finished";
  const raw = (af as Record<string, unknown>).outcome;
  const s = typeof raw === "string" ? raw : "";
  switch (s) {
    case "OUTCOME_OK":             return "finished";
    case "OUTCOME_CONDITION_FAIL": return "skipped (condition)";
    case "OUTCOME_ACTION_ERROR":   return "failed";
    case "OUTCOME_LIMIT_EXCEEDED": return "rate-limited";
    case "OUTCOME_CANCELLED":      return "cancelled";
    case "OUTCOME_SKIPPED":        return "skipped";
  }
  return "finished";
}
