/**
 * AutomationService client. Lists automations and exposes the basic
 * lifecycle controls (enable / disable / trigger) the Automations
 * page needs.
 *
 * Trace is intentionally omitted — it's a streaming RPC and best added
 * alongside the automation-detail view that consumes the stream.
 */

import { rpcCall, type RpcOptions } from "./rpc";

export interface Automation {
  id: string;
  displayName: string;
  mode: string;
  enabled: boolean;
  inFlight: number;
}

export interface ListAutomationsResponse {
  automations: Automation[];
}

interface RawAutomation {
  id?: string;
  display_name?: string; displayName?: string;
  mode?: string;
  enabled?: boolean;
  in_flight?: number; inFlight?: number;
}

function decode(r: RawAutomation): Automation {
  return {
    id:          r.id ?? "",
    displayName: r.displayName ?? r.display_name ?? r.id ?? "",
    mode:        r.mode ?? "",
    enabled:     r.enabled ?? false,
    inFlight:    r.inFlight ?? r.in_flight ?? 0,
  };
}

export async function listAutomations(opts: RpcOptions = {}): Promise<ListAutomationsResponse> {
  const res = await rpcCall<Record<string, never>, { automations?: RawAutomation[] }>(
    "switchyard.v1alpha1.AutomationService/List",
    {},
    opts,
  );
  return { automations: (res.automations ?? []).map(decode) };
}

export async function enableAutomation(id: string, opts: RpcOptions = {}): Promise<Automation> {
  const res = await rpcCall<{ id: string }, { automation?: RawAutomation }>(
    "switchyard.v1alpha1.AutomationService/Enable",
    { id },
    opts,
  );
  return decode(res.automation ?? {});
}

export async function disableAutomation(id: string, opts: RpcOptions = {}): Promise<Automation> {
  const res = await rpcCall<{ id: string }, { automation?: RawAutomation }>(
    "switchyard.v1alpha1.AutomationService/Disable",
    { id },
    opts,
  );
  return decode(res.automation ?? {});
}

/** Returns the new run id assigned by the daemon. */
export async function triggerAutomation(id: string, opts: RpcOptions = {}): Promise<string> {
  const res = await rpcCall<{ id: string }, { run_id?: string; runId?: string }>(
    "switchyard.v1alpha1.AutomationService/Trigger",
    { id },
    opts,
  );
  return res.runId ?? res.run_id ?? "";
}
