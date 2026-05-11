/**
 * system-client.ts — typed client for SystemService RPCs used by the
 * Settings Diagnostics section.
 */

export interface SubsystemHealth {
  name: string;
  ok: boolean;
  detail: string;
}

export interface HealthSummary {
  ok: boolean;
  summary: string;
  subsystems: SubsystemHealth[];
}

export interface EventStoreStats {
  sizeBytes: number;
  oldestEventAgeSeconds: number;
  snapshotCount: number;
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
    throw new Error(`system-client: ${procedure} failed: ${response.status}`);
  }
  return response.json() as Promise<TResponse>;
}

interface RawHealthResponse {
  ok?: boolean;
  summary?: string;
  subsystems?: Array<{ name?: string; ok?: boolean; detail?: string }>;
}

interface RawEventStoreStats {
  size_bytes?: string | number;
  oldest_event_age_seconds?: string | number;
  snapshot_count?: number;
}

interface RawExportResponse {
  bundle?: string; // base64-encoded bytes
  filename?: string;
  config_hash?: string;
}

export const systemClient = {
  async health(): Promise<HealthSummary> {
    const res = await postConnect<Record<string, never>, RawHealthResponse>(
      "/switchyard.v1alpha1.SystemService/Health",
      {},
    );
    return {
      ok: res.ok ?? true,
      summary: res.summary ?? "ok",
      subsystems: (res.subsystems ?? []).map((s) => ({
        name: s.name ?? "",
        ok: s.ok ?? true,
        detail: s.detail ?? "",
      })),
    };
  },

  async eventStoreStats(): Promise<EventStoreStats> {
    const res = await postConnect<Record<string, never>, RawEventStoreStats>(
      "/switchyard.v1alpha1.SystemService/GetEventStoreStats",
      {},
    );
    return {
      sizeBytes: Number(res.size_bytes ?? 0),
      oldestEventAgeSeconds: Number(res.oldest_event_age_seconds ?? 0),
      snapshotCount: res.snapshot_count ?? 0,
    };
  },

  async exportSupportBundle(): Promise<{ blob: Blob; filename: string }> {
    const res = await postConnect<Record<string, never>, RawExportResponse>(
      "/switchyard.v1alpha1.SystemService/ExportSupportBundle",
      {},
    );
    // bundle is base64-encoded bytes from proto bytes field
    const raw = res.bundle ?? "";
    const bytes = raw
      ? Uint8Array.from(atob(raw), (c) => c.charCodeAt(0))
      : new Uint8Array(0);
    return {
      blob: new Blob([bytes], { type: "application/zip" }),
      filename: res.filename ?? "switchyard-support.zip",
    };
  },
};
