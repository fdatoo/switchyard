import { StateCache } from "./state-cache";
import { CommandTracker } from "./command-tracker";
import { markDaemonReachable, markDaemonUnreachable } from "@/data/daemon-connection";

export type StreamKind = "state" | "command";
export type StreamEvent =
  | { kind: "state"; cursor: string; entityId: string; state: unknown }
  | { kind: "command_issued"; cursor: string; commandId: string; entityId: string }
  | { kind: "command_ack"; cursor: string; commandId: string; ok: boolean; error?: string };
export type StreamHandle = { close: () => void };
export type StreamFactory = (args: {
  kind: StreamKind;
  entityIds: string[];
  cursor?: string;
  onEvent: (event: StreamEvent) => void;
  onClose: () => void;
}) => StreamHandle;

export type Mux = {
  cache: StateCache;
  tracker: CommandTracker;
  subscribe: (ids: string[]) => void;
  shutdown: () => void;
  callCapability: (entityId: string, capability: string, args: Record<string, string>) => Promise<void>;
};

type Transport = {
  [key: string]: unknown;
  callCapability?: (entityId: string, capability: string, args: Record<string, string>) => Promise<{ commandId?: string } | void>;
};

export function createMultiplexer(opts: { transport?: Transport; openStream?: StreamFactory; reconnectDelayMs?: number }): Mux {
  const cache = new StateCache();
  const tracker = new CommandTracker();
  const entityIds = new Set<string>();
  const cursors: Partial<Record<StreamKind, string>> = {};
  const handles: Partial<Record<StreamKind, StreamHandle>> = {};
  const timers: Partial<Record<StreamKind, ReturnType<typeof setTimeout>>> = {};
  let stopped = false;

  const handleEvent = (event: StreamEvent) => {
    markDaemonReachable();
    if (event.kind === "state") {
      cursors.state = event.cursor;
      cache.set(event.entityId, { entityId: event.entityId, state: String(event.state) });
      return;
    }
    cursors.command = event.cursor;
    if (event.kind === "command_issued") {
      tracker.issued(event.commandId, event.entityId);
      return;
    }
    if (event.ok) {
      tracker.acked(event.commandId);
    } else {
      tracker.failed_(event.commandId, event.error ?? "command failed");
    }
  };

  const open = (kind: StreamKind) => {
    if (!opts.openStream || stopped || entityIds.size === 0) return;
    handles[kind]?.close();
    try {
      handles[kind] = opts.openStream({
        kind,
        entityIds: [...entityIds],
        cursor: cursors[kind],
        onEvent: handleEvent,
        onClose: () => {
          handles[kind] = undefined;
          markDaemonUnreachable(new Error(`${kind} stream closed`));
          if (!stopped) {
            timers[kind] = setTimeout(() => open(kind), opts.reconnectDelayMs ?? 1000);
          }
        },
      });
    } catch (error) {
      handles[kind] = undefined;
      markDaemonUnreachable(error);
      if (!stopped) {
        timers[kind] = setTimeout(() => open(kind), opts.reconnectDelayMs ?? 1000);
      }
    }
  };

  return {
    cache,
    tracker,
    subscribe: (ids: string[]) => {
      let changed = false;
      for (const id of ids) {
        if (!entityIds.has(id)) {
          entityIds.add(id);
          changed = true;
        }
      }
      if (changed) {
        open("state");
        open("command");
      }
    },
    shutdown: () => {
      stopped = true;
      for (const timer of Object.values(timers)) {
        if (timer) clearTimeout(timer);
      }
      handles.state?.close();
      handles.command?.close();
    },
    callCapability: async (entityId, capability, args) => {
      const result = await opts.transport?.callCapability?.(entityId, capability, args);
      if (result?.commandId) {
        tracker.issued(result.commandId, entityId);
      }
    },
  };
}
