/**
 * ConfigService client. Provides subscription to config changes and
 * reload notifications. Used after CommitEdit so the daemon picks up
 * the new file immediately (the file watcher would catch it too, but
 * the explicit call is deterministic and lets the UI block on completion).
 */

import { rpcCall, rpcStream, type RpcOptions } from "./rpc";

const SVC = "switchyard.v1alpha1.ConfigService";

export interface ReloadResult {
  correlationId: string;
  error?: string;
}

export async function reloadConfig(opts: RpcOptions = {}): Promise<ReloadResult> {
  const res = await rpcCall<Record<string, never>, { correlationId?: string; correlation_id?: string; error?: string }>(
    `${SVC}/Reload`, {}, opts,
  );
  return {
    correlationId: res.correlationId ?? res.correlation_id ?? "",
    error: res.error,
  };
}

export type ConfigChanged = {
  kind: "changed";
  atUnixMs: number;
  bundleHash: string;
};

export type ConfigHeartbeat = {
  kind: "heartbeat";
  atUnixMs: number;
};

export type ConfigSubscribeEvent = ConfigChanged | ConfigHeartbeat;

interface RawConfigSubscribeMessage {
  changed?: { atUnixMs?: string | number; at_unix_ms?: string | number; bundleHash?: string; bundle_hash?: string };
  heartbeat?: { atUnixMs?: string | number; at_unix_ms?: string | number };
}

function decodeConfigSubscribe(raw: RawConfigSubscribeMessage): ConfigSubscribeEvent | null {
  if (raw.changed) {
    const c = raw.changed;
    return {
      kind: "changed",
      atUnixMs: Number(c.atUnixMs ?? c.at_unix_ms ?? 0),
      bundleHash: c.bundleHash ?? c.bundle_hash ?? "",
    };
  }
  if (raw.heartbeat) {
    const h = raw.heartbeat;
    return {
      kind: "heartbeat",
      atUnixMs: Number(h.atUnixMs ?? h.at_unix_ms ?? 0),
    };
  }
  return null;
}

/** Server-streaming subscription to config-change events. */
export async function* subscribeConfig(
  opts: RpcOptions = {},
): AsyncGenerator<ConfigSubscribeEvent, void, void> {
  const stream = rpcStream<unknown, RawConfigSubscribeMessage>(
    `${SVC}/Subscribe`,
    {},
    opts,
  );
  for await (const raw of stream) {
    const ev = decodeConfigSubscribe(raw);
    if (ev) yield ev;
  }
}
