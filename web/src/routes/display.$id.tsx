/**
 * /display/:id — public ambient display renderer.
 *
 * Auth: per-display token stored in localStorage under key `sy.display.<id>.token`.
 * If token is absent, redirects to /pair?hint=<id>.
 * No Shell chrome. Uses AmbientRoot.
 */

import { useEffect, useState } from "react";
import { AmbientRoot } from "@/ambient/AmbientRoot";
import { AlertPill, AlertContext } from "@/ambient/AlertPill";
import { PlaceholderPage } from "@/shell/PlaceholderPage";

// ---------------------------------------------------------------------------
// Display service client
// ---------------------------------------------------------------------------

export interface DisplayConfig {
  id: string;
  deviceName: string;
  pageSlug: string;
  alertThreshold: "ALERT_THRESHOLD_NONE" | "ALERT_THRESHOLD_LOW" | "ALERT_THRESHOLD_MEDIUM" | "ALERT_THRESHOLD_HIGH" | "ALERT_THRESHOLD_UNSPECIFIED";
  tileOverrides?: Record<string, { width?: number; scenes?: number; metric?: number }>;
}

async function fetchDisplayConfig(id: string, token: string): Promise<DisplayConfig | null> {
  try {
    const res = await fetch("/switchyard.display.v1.DisplayService/Get", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Connect-Protocol-Version": "1",
        "Authorization": `Bearer ${token}`,
      },
      body: JSON.stringify({ id }),
    });
    if (!res.ok) return null;
    const data = await res.json() as { display?: DisplayConfig };
    return data.display ?? null;
  } catch {
    return null;
  }
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

interface DisplayPageProps {
  id: string;
}

type LoadState =
  | { status: "loading" }
  | { status: "no_token" }
  | { status: "error"; message: string }
  | { status: "ready"; config: DisplayConfig; token: string };

export function DisplayPage({ id }: DisplayPageProps) {
  const [state, setState] = useState<LoadState>({ status: "loading" });

  useEffect(() => {
    const token = localStorage.getItem(`sy.display.${id}.token`);
    if (!token) {
      setState({ status: "no_token" });
      return;
    }

    void fetchDisplayConfig(id, token).then((config) => {
      if (!config) {
        setState({ status: "error", message: "Failed to load display configuration." });
      } else {
        setState({ status: "ready", config, token });
      }
    });
  }, [id]);

  // Redirect to /pair if no token
  useEffect(() => {
    if (state.status === "no_token") {
      window.location.replace(`/pair?hint=${encodeURIComponent(id)}`);
    }
  }, [state.status, id]);

  if (state.status === "loading" || state.status === "no_token") {
    return (
      <div style={{ display: "flex", alignItems: "center", justifyContent: "center", minHeight: "100dvh", background: "#0f0a1a", color: "#ffffff" }}>
        Loading…
      </div>
    );
  }

  if (state.status === "error") {
    return (
      <div style={{ display: "flex", alignItems: "center", justifyContent: "center", minHeight: "100dvh", background: "#0f0a1a", color: "#e87a5f", padding: "2rem", textAlign: "center" }}>
        {state.message}
      </div>
    );
  }

  const { config, token } = state;
  const alertThreshold = mapThreshold(config.alertThreshold);

  return (
    <AmbientRoot displayToken={token}>
      <AlertContext alertThreshold={alertThreshold}>
        <div style={{ position: "relative", minHeight: "100dvh", padding: "var(--sy-space-6, 2rem)" }}>
          <AlertPill alertThreshold={alertThreshold} />
          <div style={{ paddingTop: "var(--sy-space-6, 2rem)" }}>
            {config.pageSlug ? (
              <PlaceholderPage title={`Display: ${config.deviceName}`} plan="Plan 07 — ambient render TODO: wire PageService.Get" />
            ) : (
              <PlaceholderPage title={`Display: ${config.deviceName}`} plan="Plan 07 — no page assigned" />
            )}
          </div>
        </div>
      </AlertContext>
    </AmbientRoot>
  );
}

function mapThreshold(raw: DisplayConfig["alertThreshold"]): "NONE" | "LOW" | "MEDIUM" | "HIGH" {
  switch (raw) {
    case "ALERT_THRESHOLD_LOW":    return "LOW";
    case "ALERT_THRESHOLD_MEDIUM": return "MEDIUM";
    case "ALERT_THRESHOLD_HIGH":   return "HIGH";
    default:                       return "NONE";
  }
}
