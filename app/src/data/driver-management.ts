/**
 * DriverManagementService client.
 *
 * Mirrors the proto shape from `proto/switchyard/driver/v1/management.proto`.
 * The daemon returns snake_case fields; we decode to a camelCase UI shape
 * so views never see wire details.
 *
 * Status string normalization: the daemon's `status` field varies by
 * proto version ("healthy" or "running" both mean running). We normalize
 * to a tight closed union so display logic (driver-state.ts) doesn't
 * need to know about historical variations.
 */

import { rpcCall, type RpcOptions } from "./rpc";

export type DriverStateName = "running" | "reconnecting" | "degraded" | "stopped" | "unknown";

export interface DriverSummary {
  id: string;
  pack: string;
  version: string;
  state: DriverStateName;
  entityCount: number;
  uptimeSeconds: number;
}

export interface ListDriversResponse {
  running: DriverSummary[];
}

interface RawDriver {
  id?: string;
  pack?: string;
  version?: string;
  status?: string;
  uptime_seconds?: number | string;
  uptimeSeconds?: number | string;
  entity_count?: number;
  entityCount?: number;
}

function normalizeState(raw: string | undefined): DriverStateName {
  switch ((raw ?? "").toLowerCase()) {
    case "running":
    case "healthy":      return "running";
    case "reconnecting": return "reconnecting";
    case "degraded":     return "degraded";
    case "stopped":      return "stopped";
    default:             return "unknown";
  }
}

/** Coerce a wire-format number (sometimes JSON-encoded as a string) to number. */
function toNumber(v: number | string | undefined): number {
  if (typeof v === "number") return v;
  if (typeof v === "string") return Number(v) || 0;
  return 0;
}

function decode(raw: RawDriver): DriverSummary {
  return {
    id: raw.id ?? "",
    pack: raw.pack ?? "",
    version: raw.version ?? "",
    state: normalizeState(raw.status),
    entityCount: raw.entityCount ?? raw.entity_count ?? 0,
    uptimeSeconds: toNumber(raw.uptimeSeconds ?? raw.uptime_seconds),
  };
}

/* RPC operations. Each is a thin typed wrapper around `rpcCall`; the
   request/response shapes match the proto exactly. */

export async function listDrivers(opts: RpcOptions = {}): Promise<ListDriversResponse> {
  const res = await rpcCall<Record<string, never>, { running?: RawDriver[] }>(
    "switchyard.driver.v1.DriverManagementService/List",
    {},
    opts,
  );
  return { running: (res.running ?? []).map(decode) };
}

export async function restartDriver(
  id: string,
  reason: string,
  opts: RpcOptions = {},
): Promise<void> {
  await rpcCall<{ id: string; reason: string }, Record<string, never>>(
    "switchyard.driver.v1.DriverManagementService/Restart",
    { id, reason },
    opts,
  );
}

export async function stopDriver(
  id: string,
  reason: string,
  opts: RpcOptions = {},
): Promise<void> {
  await rpcCall<{ id: string; reason: string }, Record<string, never>>(
    "switchyard.driver.v1.DriverManagementService/Stop",
    { id, reason },
    opts,
  );
}
