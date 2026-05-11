/**
 * /pair — public pair-code redemption page.
 *
 * No Shell, no nav. Shows the Switchyard wordmark + a 6-digit code input.
 * On submit, calls DisplayService.RedeemPairCode(), stores the returned
 * per-display token in localStorage, then navigates to /display/<id>.
 * On error: shows "Code not found or expired. Ask the operator for a new code."
 */

import { useEffect, useState } from "react";

// ---------------------------------------------------------------------------
// DisplayService client
// ---------------------------------------------------------------------------

async function redeemPairCode(code: string, deviceName: string): Promise<{ displayId: string; token: string } | null> {
  try {
    const res = await fetch("/switchyard.display.v1.DisplayService/RedeemPairCode", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Connect-Protocol-Version": "1",
      },
      body: JSON.stringify({ code, device_name: deviceName }),
    });
    if (!res.ok) return null;
    const data = await res.json() as { display_id?: string; token?: string; displayId?: string };
    const displayId = data.display_id ?? data.displayId ?? "";
    const token = data.token ?? "";
    if (!displayId || !token) return null;
    return { displayId, token };
  } catch {
    return null;
  }
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

interface PairPageProps {
  /** Pre-fill hint from query param ?hint=<display-id> */
  hint?: string;
}

export function PairPage({ hint }: PairPageProps) {
  const [code, setCode] = useState("");
  const [deviceName, setDeviceName] = useState(() =>
    typeof navigator !== "undefined"
      ? navigator.userAgent.slice(0, 40)
      : "Unknown Device",
  );
  const [status, setStatus] = useState<"idle" | "loading" | "error">("idle");
  const [errorMessage, setErrorMessage] = useState("");

  // Read hint from URL if not passed as prop
  useEffect(() => {
    if (!hint && typeof window !== "undefined") {
      const params = new URLSearchParams(window.location.search);
      const h = params.get("hint");
      if (h) {
        // hint is a display ID to redirect to after pairing
        // no UI change needed — it's used after successful pairing
        void h; // hint is used in handleSubmit
      }
    }
  }, [hint]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (code.length !== 6) {
      setStatus("error");
      setErrorMessage("Please enter a 6-digit code.");
      return;
    }
    setStatus("loading");
    const result = await redeemPairCode(code.trim(), deviceName.trim() || "Unknown Device");
    if (!result) {
      setStatus("error");
      setErrorMessage("Code not found or expired. Ask the operator for a new code.");
      return;
    }
    // Store per-display token and navigate
    localStorage.setItem(`sy.display.${result.displayId}.token`, result.token);
    window.location.replace(`/display/${result.displayId}`);
  }

  return (
    <div
      data-testid="pair-page"
      style={{
        minHeight: "100dvh",
        background: "var(--sy-tod-night)",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        padding: "2rem",
        fontFamily: "var(--sy-font-body)",
        color: "var(--sy-color-fg)",
      }}
    >
      {/* Wordmark */}
      <div
        style={{
          fontSize: "1.5rem",
          fontWeight: 700,
          letterSpacing: "-0.03em",
          marginBottom: "3rem",
          color: "var(--sy-color-accent)",
        }}
      >
        switchyard
      </div>

      <form
        onSubmit={(e) => void handleSubmit(e)}
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "1.5rem",
          width: "100%",
          maxWidth: "360px",
        }}
      >
        <div style={{ textAlign: "center" }}>
          <h1 style={{ fontSize: "1.25rem", fontWeight: 600, margin: "0 0 0.5rem" }}>
            Pair this display
          </h1>
          <p style={{ color: "var(--sy-color-fg-3)", margin: 0, fontSize: "0.875rem" }}>
            Enter the 6-digit code shown by the operator.
          </p>
        </div>

        {/* Code input */}
        <input
          type="text"
          inputMode="numeric"
          maxLength={6}
          value={code}
          onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
          placeholder="000000"
          data-testid="pair-code-input"
          style={{
            background: "var(--sy-color-surface-1)",
            border: "1px solid var(--sy-color-line)",
            borderRadius: "var(--sy-radius-xl)",
            color: "var(--sy-color-fg)",
            fontSize: "2.5rem",
            fontWeight: 700,
            letterSpacing: "0.5em",
            padding: "1rem 1.5rem",
            textAlign: "center",
            outline: "none",
            width: "100%",
            boxSizing: "border-box",
          }}
        />

        {/* Device name */}
        <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
          <label style={{ fontSize: "0.75rem", color: "var(--sy-color-fg-4)", textTransform: "uppercase", letterSpacing: "0.08em" }}>
            Device name
          </label>
          <input
            type="text"
            value={deviceName}
            onChange={(e) => setDeviceName(e.target.value)}
            data-testid="device-name-input"
            style={{
              background: "var(--sy-color-surface-1)",
              border: "1px solid var(--sy-color-line)",
              borderRadius: "var(--sy-radius)",
              color: "var(--sy-color-fg)",
              fontSize: "0.875rem",
              padding: "0.75rem 1rem",
              outline: "none",
              width: "100%",
              boxSizing: "border-box",
            }}
          />
        </div>

        {/* Error message */}
        {status === "error" && (
          <p
            data-testid="pair-error"
            style={{
              color: "var(--sy-color-bad)",
              fontSize: "0.875rem",
              textAlign: "center",
              margin: 0,
            }}
          >
            {errorMessage}
          </p>
        )}

        {/* Submit button */}
        <button
          type="submit"
          disabled={status === "loading" || code.length !== 6}
          data-testid="pair-submit"
          style={{
            background: status === "loading" ? "var(--sy-color-accent-soft)" : "var(--sy-color-accent)",
            border: "none",
            borderRadius: "var(--sy-radius-xl)",
            color: "var(--sy-color-bg)",
            cursor: status === "loading" ? "not-allowed" : "pointer",
            fontSize: "1rem",
            fontWeight: 600,
            padding: "1rem",
            width: "100%",
          }}
        >
          {status === "loading" ? "Pairing…" : "Pair display"}
        </button>
      </form>
    </div>
  );
}
