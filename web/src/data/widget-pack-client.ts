/**
 * widget-pack-client.ts — typed client wrapper for WidgetPackService.
 * Calls widgetpack.ListWidgetPacks and widgetpack.Install RPCs.
 */

export type SignatureStatus = "verified" | "unverified" | "pending";

export interface InstalledPack {
  name: string;
  version: string;
  sha256: string;
  signature: SignatureStatus;
  signerIdentity: string;
  ociRef: string;
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
    throw new Error(`widget-pack-client: ${procedure} failed: ${response.status}`);
  }
  return response.json() as Promise<TResponse>;
}

interface RawInstalledPack {
  name?: string;
  version?: string;
  sha256?: string;
  signature?: string;
  signer_identity?: string;
  bundle_url?: string;
}

function toInstalledPack(raw: RawInstalledPack): InstalledPack {
  const sig = raw.signature;
  const status: SignatureStatus =
    sig === "SIGNATURE_STATUS_VERIFIED"
      ? "verified"
      : sig === "SIGNATURE_STATUS_UNVERIFIED"
        ? "unverified"
        : "pending";
  return {
    name: raw.name ?? "",
    version: raw.version ?? "",
    sha256: raw.sha256 ?? "",
    signature: status,
    signerIdentity: raw.signer_identity ?? "",
    ociRef: raw.bundle_url ?? `${raw.name ?? "unknown"}:${raw.version ?? "latest"}`,
  };
}

export const widgetPackClient = {
  async listInstalledPacks(): Promise<InstalledPack[]> {
    const res = await postConnect<
      Record<string, never>,
      { packs?: RawInstalledPack[] }
    >("/switchyard.v1alpha1.WidgetPackService/List", {});
    return (res.packs ?? []).map(toInstalledPack);
  },

  async installPack(ociRef: string): Promise<InstalledPack> {
    const res = await postConnect<{ ref: string }, { pack?: RawInstalledPack }>(
      "/switchyard.v1alpha1.WidgetPackService/Install",
      { ref: ociRef },
    );
    return toInstalledPack(res.pack ?? {});
  },
};

/**
 * useInstalledPacks — React hook that calls listInstalledPacks and returns the
 * result. Used by WidgetPacks.tsx.
 */
import { useEffect, useState } from "react";

export function useInstalledPacks(): {
  packs: InstalledPack[];
  loading: boolean;
  error: string | null;
  installPack: (ociRef: string) => Promise<void>;
  refresh: () => void;
} {
  const [packs, setPacks] = useState<InstalledPack[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    widgetPackClient
      .listInstalledPacks()
      .then((p) => {
        if (!cancelled) setPacks(p);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Failed to load packs");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [tick]);

  const installPack = async (ociRef: string) => {
    await widgetPackClient.installPack(ociRef);
    setTick((t) => t + 1);
  };

  const refresh = () => setTick((t) => t + 1);

  return { packs, loading, error, installPack, refresh };
}
