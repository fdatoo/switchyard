/**
 * /_authed/displays — Display list + "Pair new display" affordance.
 * Fetches DisplayService.List() and renders a table of paired displays.
 * "Pair new display" opens a modal showing the 6-digit code with a countdown.
 */

import { useEffect, useState, useRef } from "react";
import type { DisplayConfig } from "@/routes/display.$id";

// ---------------------------------------------------------------------------
// Service client
// ---------------------------------------------------------------------------

async function listDisplays(): Promise<DisplayConfig[]> {
  try {
    const res = await fetch("/switchyard.display.v1.DisplayService/List", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
      body: JSON.stringify({}),
    });
    if (!res.ok) return [];
    const data = await res.json() as { displays?: DisplayConfig[] };
    return data.displays ?? [];
  } catch { return []; }
}

async function pairDisplay(): Promise<{ code: string; expiresAt: number } | null> {
  try {
    const res = await fetch("/switchyard.display.v1.DisplayService/Pair", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
      body: JSON.stringify({}),
    });
    if (!res.ok) return null;
    const data = await res.json() as { code?: string; expires_at?: number; expiresAt?: number };
    const expiresAt = data.expires_at ?? data.expiresAt ?? 0;
    return { code: data.code ?? "", expiresAt };
  } catch { return null; }
}

async function unpairDisplay(id: string): Promise<boolean> {
  try {
    const res = await fetch("/switchyard.display.v1.DisplayService/Unpair", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
      body: JSON.stringify({ id }),
    });
    return res.ok;
  } catch { return false; }
}

// ---------------------------------------------------------------------------
// Pairing modal
// ---------------------------------------------------------------------------

interface PairingModalProps {
  onClose: () => void;
}

function PairingModal({ onClose }: PairingModalProps) {
  const [state, setState] = useState<"loading" | "showing" | "error">("loading");
  const [code, setCode] = useState("");
  const [secondsLeft, setSecondsLeft] = useState(300);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    void pairDisplay().then((result) => {
      if (!result) { setState("error"); return; }
      setCode(result.code);
      const totalSecs = Math.max(0, Math.round((result.expiresAt - Date.now() / 1000)));
      setSecondsLeft(totalSecs > 0 ? totalSecs : 300);
      setState("showing");
    });
  }, []);

  useEffect(() => {
    if (state !== "showing") return;
    intervalRef.current = setInterval(() => {
      setSecondsLeft((s) => {
        if (s <= 1) { onClose(); return 0; }
        return s - 1;
      });
    }, 1000);
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
  }, [state, onClose]);

  const minutes = String(Math.floor(secondsLeft / 60)).padStart(2, "0");
  const seconds = String(secondsLeft % 60).padStart(2, "0");

  return (
    <div
      data-testid="pairing-modal"
      style={{
        position: "fixed", inset: 0, background: "var(--sy-color-overlay)",
        display: "flex", alignItems: "center", justifyContent: "center", zIndex: 500,
      }}
      onClick={onClose}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          background: "var(--sy-color-surface-1)",
          border: "1px solid var(--sy-color-line)",
          borderRadius: "var(--sy-radius-xl, 24px)",
          padding: "2rem",
          maxWidth: "380px",
          width: "90%",
          textAlign: "center",
        }}
      >
        {state === "loading" && <p>Generating code…</p>}
        {state === "error"   && <p style={{ color: "var(--sy-color-bad)" }}>Failed to generate code.</p>}
        {state === "showing" && (
          <>
            <p style={{ fontSize: "0.875rem", color: "var(--sy-color-fg-3)", marginBottom: "1rem" }}>
              Enter this code on the display device at <strong>/pair</strong>:
            </p>
            <div
              data-testid="pair-code-display"
              style={{ fontSize: "3rem", fontWeight: 700, letterSpacing: "0.5em", color: "var(--sy-color-accent)", margin: "1rem 0" }}
            >
              {code}
            </div>
            <p style={{ color: "var(--sy-color-fg-3)", fontSize: "0.875rem" }}>
              Expires in {minutes}:{seconds}
            </p>
          </>
        )}
        <button
          data-testid="pairing-modal-close"
          onClick={onClose}
          style={{
            marginTop: "1.5rem",
            background: "transparent",
            border: "1px solid var(--sy-color-line)",
            borderRadius: "var(--sy-radius)",
            color: "var(--sy-color-fg-3)",
            cursor: "pointer",
            padding: "0.5rem 1.5rem",
          }}
        >
          Close
        </button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Display list page
// ---------------------------------------------------------------------------

const THRESHOLD_LABELS: Record<string, string> = {
  ALERT_THRESHOLD_NONE:        "None",
  ALERT_THRESHOLD_LOW:         "Low",
  ALERT_THRESHOLD_MEDIUM:      "Medium",
  ALERT_THRESHOLD_HIGH:        "High",
  ALERT_THRESHOLD_UNSPECIFIED: "—",
};

export function DisplaysIndex() {
  const [displays, setDisplays] = useState<DisplayConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [pairingOpen, setPairingOpen] = useState(false);

  async function load() {
    setLoading(true);
    setDisplays(await listDisplays());
    setLoading(false);
  }

  useEffect(() => { void load(); }, []);

  async function handleUnpair(id: string, name: string) {
    if (!confirm(`Unpair "${name}"? The display will lose access.`)) return;
    await unpairDisplay(id);
    void load();
  }

  return (
    <div style={{ padding: "var(--sy-space-5, 1.5rem)" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: "var(--sy-space-4, 1rem)" }}>
        <h1 style={{ fontSize: "1.25rem", fontWeight: 600, margin: 0 }}>Displays</h1>
        <button
          data-testid="pair-new-display-btn"
          onClick={() => setPairingOpen(true)}
          style={{
            background: "var(--sy-color-accent)",
            border: "none",
            borderRadius: "var(--sy-radius, 12px)",
            color: "var(--sy-color-bg)",
            cursor: "pointer",
            fontSize: "0.875rem",
            fontWeight: 500,
            padding: "0.5rem 1rem",
          }}
        >
          Pair new display
        </button>
      </div>

      {loading && <p style={{ color: "var(--sy-color-fg-3)" }}>Loading…</p>}

      {!loading && displays.length === 0 && (
        <p data-testid="no-displays-msg" style={{ color: "var(--sy-color-fg-3)" }}>
          No displays paired yet. Click &ldquo;Pair new display&rdquo; to get started.
        </p>
      )}

      {displays.length > 0 && (
        <table
          data-testid="displays-table"
          style={{ width: "100%", borderCollapse: "collapse", fontSize: "0.875rem" }}
        >
          <thead>
            <tr style={{ textAlign: "left", color: "var(--sy-color-fg-3)", borderBottom: "1px solid var(--sy-color-line)" }}>
              <th style={{ padding: "0.5rem 0.75rem" }}>Device</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Page</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Alert Threshold</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {displays.map((d) => (
              <tr key={d.id} style={{ borderBottom: "1px solid var(--sy-color-line-soft)" }}>
                <td data-testid={`display-name-${d.id}`} style={{ padding: "0.75rem" }}>{d.deviceName}</td>
                <td style={{ padding: "0.75rem", color: d.pageSlug ? "inherit" : "var(--sy-color-fg-3)" }}>
                  {d.pageSlug || "—"}
                </td>
                <td style={{ padding: "0.75rem" }}>{THRESHOLD_LABELS[d.alertThreshold] ?? "—"}</td>
                <td style={{ padding: "0.75rem", display: "flex", gap: "0.5rem" }}>
                  <a
                    href={`/_authed/displays/${d.id}`}
                    data-testid={`configure-${d.id}`}
                    style={{ color: "var(--sy-color-accent)", textDecoration: "none", fontSize: "0.8125rem" }}
                  >
                    Configure
                  </a>
                  <button
                    data-testid={`unpair-${d.id}`}
                    onClick={() => void handleUnpair(d.id, d.deviceName)}
                    style={{
                      background: "transparent",
                      border: "none",
                      color: "var(--sy-color-bad)",
                      cursor: "pointer",
                      fontSize: "0.8125rem",
                      padding: 0,
                    }}
                  >
                    Unpair
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {pairingOpen && (
        <PairingModal onClose={() => { setPairingOpen(false); void load(); }} />
      )}
    </div>
  );
}
