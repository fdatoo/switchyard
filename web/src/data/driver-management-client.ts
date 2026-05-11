/**
 * driver-management-client.ts — typed ConnectRPC client wrapper for
 * DriverManagementService (proto: switchyard/driver/v1/management.proto).
 *
 * Follows the same Connect-ES client patterns established in data/*.
 */

/**
 * DriverSummary mirrors the proto DriverSummary message shape for use in
 * React components without importing protobuf generated code directly.
 */
export interface DriverSummary {
  id: string;
  pack: string;
  version: string;
  /** "healthy" | "reconnecting" | "degraded" */
  status: string;
  uptimeSeconds: number;
  pid: number;
  socket: string;
  configFile: string;
  otelSpan: string;
  entityCount: number;
  /** TODO: compute from real metrics */
  eventsPerDay: number;
  /** TODO: compute from real metrics */
  lastCmdAckMs: number;
  /** TODO: compute from real metrics */
  reconnectsToday: number;
  reconnectingSince: string;
}

export interface RegistryDriver {
  id: string;
  pack: string;
  version: string;
  /** "available" | "update_available" */
  status: string;
}

export interface ListDriversResponse {
  running: DriverSummary[];
  available: RegistryDriver[];
}

async function postConnect<TRequest, TResponse>(
  procedure: string,
  body: TRequest,
): Promise<TResponse> {
  const response = await fetch(procedure, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      "Connect-Protocol-Version": "1",
    },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    throw new Error(`driver-management-client: ${procedure} failed: ${response.status}`);
  }
  return response.json() as Promise<TResponse>;
}

function toCamel(snake: Record<string, unknown>): DriverSummary {
  return {
    id: (snake["id"] as string) ?? "",
    pack: (snake["pack"] as string) ?? "",
    version: (snake["version"] as string) ?? "",
    status: (snake["status"] as string) ?? "healthy",
    uptimeSeconds: (snake["uptime_seconds"] as number) ?? 0,
    pid: (snake["pid"] as number) ?? 0,
    socket: (snake["socket"] as string) ?? "",
    configFile: (snake["config_file"] as string) ?? "",
    otelSpan: (snake["otel_span"] as string) ?? "",
    entityCount: (snake["entity_count"] as number) ?? 0,
    eventsPerDay: (snake["events_per_day"] as number) ?? 0,
    lastCmdAckMs: (snake["last_cmd_ack_ms"] as number) ?? 0,
    reconnectsToday: (snake["reconnects_today"] as number) ?? 0,
    reconnectingSince: (snake["reconnecting_since"] as string) ?? "",
  };
}

export const driverClient = {
  async list(): Promise<ListDriversResponse> {
    const res = await postConnect<Record<string, never>, {
      running?: Record<string, unknown>[];
      available?: Record<string, unknown>[];
    }>("/switchyard.driver.v1.DriverManagementService/List", {});
    return {
      running: (res.running ?? []).map((d) => toCamel(d)),
      available: (res.available ?? []).map((d) => ({
        id: (d["id"] as string) ?? "",
        pack: (d["pack"] as string) ?? "",
        version: (d["version"] as string) ?? "",
        status: (d["status"] as string) ?? "available",
      })),
    };
  },

  async get(id: string): Promise<DriverSummary> {
    const res = await postConnect<{ id: string }, { driver?: Record<string, unknown> }>(
      "/switchyard.driver.v1.DriverManagementService/Get",
      { id },
    );
    return toCamel(res.driver ?? {});
  },

  async restart(id: string, reason: string): Promise<void> {
    await postConnect<{ id: string; reason: string }, Record<string, never>>(
      "/switchyard.driver.v1.DriverManagementService/Restart",
      { id, reason },
    );
  },

  async stop(id: string, reason: string): Promise<void> {
    await postConnect<{ id: string; reason: string }, Record<string, never>>(
      "/switchyard.driver.v1.DriverManagementService/Stop",
      { id, reason },
    );
  },

  async logs(id: string, lastN: number): Promise<string[]> {
    const res = await postConnect<{ id: string; last_n: number }, { lines?: string[] }>(
      "/switchyard.driver.v1.DriverManagementService/Logs",
      { id, last_n: lastN },
    );
    return res.lines ?? [];
  },
};
