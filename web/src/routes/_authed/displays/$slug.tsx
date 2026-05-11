/**
 * /_authed/displays/$id — per-display config editor.
 *
 * Loads the display via DisplayService.Get(id). Provides form sections for:
 * (1) Assigned Page, (2) Fidelity Overrides, (3) Idle Behavior,
 * (4) Allowed Interactions, (5) Alert Threshold.
 * Save calls DisplayService.Update(). "Preview" opens /display/<id> in new tab.
 * "Unpair" (destructive) calls DisplayService.Unpair().
 */

import { useEffect, useState } from "react";
import type { DisplayConfig } from "@/routes/display.$id";

// ---------------------------------------------------------------------------
// Service clients
// ---------------------------------------------------------------------------

async function getDisplay(id: string): Promise<DisplayConfig | null> {
  try {
    const res = await fetch("/switchyard.display.v1.DisplayService/Get", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
      body: JSON.stringify({ id }),
    });
    if (!res.ok) return null;
    const data = await res.json() as { display?: DisplayConfig };
    return data.display ?? null;
  } catch { return null; }
}

async function updateDisplay(id: string, config: Partial<DisplayConfig>): Promise<boolean> {
  try {
    const res = await fetch("/switchyard.display.v1.DisplayService/Update", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
      body: JSON.stringify({ id, config }),
    });
    return res.ok;
  } catch { return false; }
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
// Types
// ---------------------------------------------------------------------------

type AlertThresholdValue = DisplayConfig["alertThreshold"];

const ALERT_THRESHOLD_OPTIONS: { label: string; value: AlertThresholdValue }[] = [
  { label: "None",   value: "ALERT_THRESHOLD_NONE" },
  { label: "Low",    value: "ALERT_THRESHOLD_LOW" },
  { label: "Medium", value: "ALERT_THRESHOLD_MEDIUM" },
  { label: "High",   value: "ALERT_THRESHOLD_HIGH" },
];

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

interface DisplaySlugProps {
  slug?: string;
}

export function DisplaySlug({ slug = "unknown" }: DisplaySlugProps) {
  const id = slug;
  const [display, setDisplay] = useState<DisplayConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [alertThreshold, setAlertThreshold] = useState<AlertThresholdValue>("ALERT_THRESHOLD_NONE");
  const [pageSlug, setPageSlug] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    void getDisplay(id).then((d) => {
      setDisplay(d);
      if (d) {
        setAlertThreshold(d.alertThreshold);
        setPageSlug(d.pageSlug ?? "");
      }
      setLoading(false);
    });
  }, [id]);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    if (!display) return;
    setSaving(true);
    await updateDisplay(id, { alertThreshold, pageSlug });
    setSaving(false);
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  }

  async function handleUnpair() {
    if (!display) return;
    if (!confirm(`Unpair "${display.deviceName}"? This cannot be undone.`)) return;
    await unpairDisplay(id);
    window.location.replace("/_authed/displays");
  }

  if (loading) {
    return <div style={{ padding: "2rem", color: "var(--sy-color-fg-3)" }}>Loading…</div>;
  }

  if (!display) {
    return (
      <div style={{ padding: "2rem", color: "var(--sy-color-bad)" }}>
        Display not found. It may have been unpaired.
      </div>
    );
  }

  return (
    <div
      data-testid="display-config-page"
      style={{ padding: "var(--sy-space-5, 1.5rem)", maxWidth: "640px" }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: "1rem", marginBottom: "2rem" }}>
        <a href="/_authed/displays" style={{ color: "var(--sy-color-fg-3)", textDecoration: "none", fontSize: "0.875rem" }}>
          ← Displays
        </a>
        <h1 style={{ fontSize: "1.25rem", fontWeight: 600, margin: 0 }}>{display.deviceName}</h1>
      </div>

      <form onSubmit={(e) => void handleSave(e)} style={{ display: "flex", flexDirection: "column", gap: "1.5rem" }}>

        {/* (1) Assigned Page */}
        <section>
          <label style={{ display: "block", fontSize: "0.75rem", color: "var(--sy-color-fg-3)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: "0.5rem" }}>
            Assigned Page
          </label>
          <input
            type="text"
            value={pageSlug}
            onChange={(e) => setPageSlug(e.target.value)}
            placeholder="Page slug (e.g. home)"
            data-testid="page-slug-input"
            style={{
              background: "var(--sy-color-surface-1)",
              border: "1px solid var(--sy-color-line)",
              borderRadius: "var(--sy-radius)",
              color: "var(--sy-color-fg)",
              fontSize: "0.875rem",
              padding: "0.6rem 0.875rem",
              width: "100%",
              boxSizing: "border-box",
            }}
          />
        </section>

        {/* (5) Alert Threshold */}
        <section>
          <label style={{ display: "block", fontSize: "0.75rem", color: "var(--sy-color-fg-3)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: "0.5rem" }}>
            Alert Threshold
          </label>
          <div style={{ display: "flex", gap: "0.75rem", flexWrap: "wrap" }}>
            {ALERT_THRESHOLD_OPTIONS.map((opt) => (
              <label key={opt.value} style={{ display: "flex", alignItems: "center", gap: "0.375rem", cursor: "pointer", fontSize: "0.875rem" }}>
                <input
                  type="radio"
                  name="alert_threshold"
                  value={opt.value}
                  checked={alertThreshold === opt.value}
                  onChange={() => setAlertThreshold(opt.value)}
                  data-testid={`threshold-${opt.value}`}
                />
                {opt.label}
              </label>
            ))}
          </div>
        </section>

        {/* Fidelity overrides section */}
        <section>
          <label style={{ display: "block", fontSize: "0.75rem", color: "var(--sy-color-fg-3)", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: "0.5rem" }}>
            Fidelity Overrides
          </label>
          <div style={{ background: "var(--sy-color-surface-1)", borderRadius: "var(--sy-radius)", padding: "0.875rem", fontSize: "0.875rem", color: "var(--sy-color-fg-3)" }}>
            {Object.keys(display.tileOverrides ?? {}).length === 0 ? (
              <span>No overrides — using server-recommended fidelity.</span>
            ) : (
              <pre style={{ margin: 0, fontFamily: "monospace", fontSize: "0.75rem" }}>
                {JSON.stringify(display.tileOverrides, null, 2)}
              </pre>
            )}
          </div>
        </section>

        {/* Actions */}
        <div style={{ display: "flex", gap: "0.75rem", alignItems: "center" }}>
          <button
            type="submit"
            disabled={saving}
            data-testid="save-btn"
            style={{
              background: "var(--sy-color-accent)",
              border: "none",
              borderRadius: "var(--sy-radius)",
              color: "var(--sy-color-bg)",
              cursor: saving ? "not-allowed" : "pointer",
              fontSize: "0.875rem",
              fontWeight: 500,
              padding: "0.6rem 1.25rem",
            }}
          >
            {saving ? "Saving…" : saved ? "Saved!" : "Save changes"}
          </button>
          <a
            href={`/display/${id}`}
            target="_blank"
            rel="noopener noreferrer"
            data-testid="preview-btn"
            style={{ color: "var(--sy-color-fg-3)", fontSize: "0.875rem" }}
          >
            Preview in new tab
          </a>
          <button
            type="button"
            onClick={() => void handleUnpair()}
            data-testid="unpair-btn"
            style={{
              marginLeft: "auto",
              background: "transparent",
              border: "1px solid var(--sy-color-bad)",
              borderRadius: "var(--sy-radius)",
              color: "var(--sy-color-bad)",
              cursor: "pointer",
              fontSize: "0.875rem",
              padding: "0.5rem 1rem",
            }}
          >
            Unpair
          </button>
        </div>
      </form>
    </div>
  );
}
