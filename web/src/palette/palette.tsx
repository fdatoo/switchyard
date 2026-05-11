/**
 * palette.tsx
 * Command palette modal — default and active states.
 * Built without Radix Dialog (no dependency installed) using a portal-based div overlay.
 * Uses Surface primitive from LanguagePrimitives and --sy-* tokens only.
 * UI v2 Plan 05.
 */
import { useEffect, useRef, useState, useCallback } from "react";
import type { CSSProperties } from "react";
import { parsePaletteInput } from "./palette-state";
import type { Verb } from "./palette-state";
import { useRecentlyUsed, usePaletteCliPreview } from "./recently-used";
import { useMcpConfigured } from "./use-mcp-configured";

// ─── Types ───────────────────────────────────────────────────────────────────

interface PaletteProps {
  open: boolean;
  onClose: () => void;
  catalog: Verb[];
  /** navigate function — injected so palette doesn't depend on router directly */
  navigate?: (path: string) => void;
}

// ─── Jump-to items (static) ──────────────────────────────────────────────────

const JUMP_TO_ITEMS = [
  { label: "Activity", annotation: "live event feed", shortcut: "⌘3", href: "/_authed/activity" },
  { label: "Settings › Drivers", annotation: "", shortcut: "", href: "/_authed/settings/drivers" },
  { label: "Settings › Account · passkeys", annotation: "", shortcut: "", href: "/_authed/settings/account" },
];

// ─── Palette component ───────────────────────────────────────────────────────

export function Palette({ open, onClose, catalog, navigate }: PaletteProps) {
  const [input, setInput] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);
  const recentlyUsed = useRecentlyUsed(5);
  const [cliPreviewOn] = usePaletteCliPreview();
  const mcpConfigured = useMcpConfigured();

  // Focus the input when the palette opens.
  useEffect(() => {
    if (open) {
      setInput("");
      setTimeout(() => inputRef.current?.focus(), 0);
    }
  }, [open]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
      }
    },
    [onClose],
  );

  const handleBackdropClick = useCallback(
    (e: React.MouseEvent) => {
      if (e.target === e.currentTarget) {
        // Only close on backdrop click if it's not a click inside the palette.
        // Per plan: Esc only closes.
        // So we deliberately do NOT close here.
      }
    },
    [],
  );

  if (!open) return null;

  const state = parsePaletteInput(input, catalog);

  return (
    <div
      data-testid="palette-backdrop"
      onClick={handleBackdropClick}
      style={backdropStyle}
    >
      <div
        data-testid="palette-modal"
        role="dialog"
        aria-modal="true"
        aria-label="Command palette"
        style={modalStyle}
      >
        {/* Input row */}
        <div style={inputRowStyle}>
          <span style={cmdKStyle}>⌘K</span>
          <input
            ref={inputRef}
            type="text"
            role="textbox"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Search or run a command..."
            style={inputStyle}
            autoComplete="off"
            spellCheck={false}
          />
          <kbd style={escBadgeStyle}>esc</kbd>
        </div>

        {/* Resolved-as row (active state) */}
        {state.kind === "resolved" && (
          <div style={resolvedRowStyle}>
            <span style={{ display: "flex", gap: "8px", alignItems: "center", flexWrap: "wrap" }}>
              <span
                data-testid="palette-resolved-verb"
                style={verbChipStyle}
              >
                {state.verb.name}
              </span>
              {Object.entries(state.filledArgs).map(([k, v]) => (
                <span key={k} style={filledArgChipStyle}>
                  {k} <span style={{ color: "var(--sy-color-accent)" }}>:{v}</span>
                </span>
              ))}
              {state.missingRequired.map((a) => (
                <span key={a.name} style={missingRequiredChipStyle}>
                  {a.name} :?
                </span>
              ))}
              {state.missingOptional.map((a) => (
                <span key={a.name} style={missingOptionalChipStyle}>
                  {a.name} :?
                </span>
              ))}
            </span>
            {cliPreviewOn && (
              <span
                data-testid="palette-cli-preview"
                style={cliPreviewStyle}
              >
                {state.cliPreview}
              </span>
            )}
          </div>
        )}

        {/* Partial state: show candidates */}
        {state.kind === "partial" && state.verbCandidates.length > 0 && (
          <div style={sectionStyle}>
            <div style={sectionHeaderStyle}>MATCHES</div>
            {state.verbCandidates.slice(0, 8).map((v) => (
              <div key={v.name} style={itemStyle}>
                <span style={{ color: "var(--sy-color-fg)" }}>{v.name}</span>
                <span style={{ color: "var(--sy-color-fg-4)", fontSize: "12px" }}>
                  {v.description}
                </span>
              </div>
            ))}
          </div>
        )}

        {/* Default state sections */}
        {state.kind === "empty" && (
          <>
            {/* Recently Used */}
            {recentlyUsed.length > 0 && (
              <div style={sectionStyle}>
                <div style={sectionHeaderStyle}>
                  RECENTLY USED
                  <span style={sectionAnnotationStyle}>LAST 7 DAYS</span>
                </div>
                {recentlyUsed.map((r, i) => (
                  <div key={i} style={itemStyle}>
                    <span style={{ color: "var(--sy-color-fg)" }}>{r.verbName}</span>
                    <span style={{ color: "var(--sy-color-fg-4)", fontSize: "12px" }}>
                      {Object.entries(r.args)
                        .map(([k, v]) => `${k}:${v}`)
                        .join(" · ")}
                    </span>
                    <span style={{ marginLeft: "auto", color: "var(--sy-color-fg-4)", fontSize: "12px" }}>
                      {relativeTime(r.ranAt)}
                    </span>
                  </div>
                ))}
              </div>
            )}

            {/* Jump To */}
            <div style={sectionStyle}>
              <div style={sectionHeaderStyle}>JUMP TO</div>
              {JUMP_TO_ITEMS.map((item) => (
                <div
                  key={item.href}
                  style={itemStyle}
                  onClick={() => {
                    if (navigate) navigate(item.href);
                    onClose();
                  }}
                >
                  <span
                    style={{
                      width: "28px",
                      height: "28px",
                      borderRadius: "var(--sy-radius-sm)",
                      background: "color-mix(in srgb, var(--sy-color-purple) 20%, transparent)",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      flexShrink: 0,
                    }}
                  />
                  <span style={{ color: "var(--sy-color-fg)" }}>{item.label}</span>
                  {item.annotation && (
                    <span style={{ color: "var(--sy-color-fg-4)", fontSize: "12px" }}>
                      {item.annotation}
                    </span>
                  )}
                  {item.shortcut && (
                    <kbd style={{ ...escBadgeStyle, marginLeft: "auto" }}>{item.shortcut}</kbd>
                  )}
                </div>
              ))}
            </div>

            {/* Ask (MCP-gated) */}
            {mcpConfigured && (
              <div style={sectionStyle}>
                <div style={sectionHeaderStyle}>ASK</div>
                <div
                  style={itemStyle}
                  onClick={() => {
                    const q = input ? `?q=${encodeURIComponent(input)}` : "";
                    if (navigate) navigate(`/ask${q}`);
                    onClose();
                  }}
                >
                  <span style={{ color: "var(--sy-color-fg)" }}>
                    Ask the Switchyard agent…
                  </span>
                </div>
              </div>
            )}
          </>
        )}

        {/* Keyboard hint bar */}
        <div style={hintBarStyle}>
          <span>↵ open</span>
          <span>·</span>
          <span>↑↓ navigate</span>
          <span>·</span>
          <span>⇥ fill arg</span>
          <span>·</span>
          <span>⇧↵ run + stay</span>
          <span>·</span>
          <span>esc close</span>
        </div>
      </div>
    </div>
  );
}

// ─── Styles ──────────────────────────────────────────────────────────────────

const backdropStyle: CSSProperties = {
  position: "fixed",
  inset: 0,
  background: "rgba(0,0,0,0.45)",
  zIndex: 9000,
  display: "flex",
  justifyContent: "center",
  alignItems: "flex-start",
  paddingTop: "56px",
};

const modalStyle: CSSProperties = {
  width: "100%",
  maxWidth: "580px",
  background: "var(--sy-color-surface-1)",
  borderRadius: "var(--sy-radius-lg)",
  boxShadow: "var(--sy-shadow)",
  border: "1px solid var(--sy-color-line)",
  overflow: "hidden",
  display: "flex",
  flexDirection: "column",
};

const inputRowStyle: CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: "10px",
  padding: "14px 20px",
  borderBottom: "1px solid var(--sy-color-line)",
};

const cmdKStyle: CSSProperties = {
  fontFamily: "var(--sy-font-numeric)",
  fontSize: "13px",
  color: "var(--sy-color-fg-4)",
  flexShrink: 0,
};

const inputStyle: CSSProperties = {
  flex: 1,
  border: "none",
  outline: "none",
  background: "transparent",
  color: "var(--sy-color-fg)",
  fontSize: "15px",
  fontFamily: "var(--sy-font-body)",
};

const escBadgeStyle: CSSProperties = {
  fontFamily: "var(--sy-font-numeric)",
  fontSize: "11px",
  padding: "2px 6px",
  background: "var(--sy-color-surface-2)",
  borderRadius: "var(--sy-radius-sm)",
  color: "var(--sy-color-fg-4)",
  border: "1px solid var(--sy-color-line)",
  flexShrink: 0,
};

const resolvedRowStyle: CSSProperties = {
  display: "flex",
  alignItems: "center",
  justifyContent: "space-between",
  padding: "10px 20px",
  borderBottom: "1px solid var(--sy-color-line)",
  background: "var(--sy-color-surface-2)",
  gap: "8px",
  flexWrap: "wrap",
};

const verbChipStyle: CSSProperties = {
  padding: "3px 10px",
  borderRadius: "var(--sy-radius-pill)",
  background: "var(--sy-color-accent)",
  color: "var(--sy-color-bg)",
  fontSize: "12px",
  fontWeight: 600,
};

const filledArgChipStyle: CSSProperties = {
  padding: "3px 10px",
  borderRadius: "var(--sy-radius-pill)",
  background: "var(--sy-color-surface-3)",
  color: "var(--sy-color-fg-2)",
  fontSize: "12px",
  border: "1px solid var(--sy-color-line)",
};

const missingRequiredChipStyle: CSSProperties = {
  padding: "3px 10px",
  borderRadius: "var(--sy-radius-pill)",
  background: "transparent",
  color: "var(--sy-color-accent)",
  fontSize: "12px",
  border: "1.5px dashed var(--sy-color-accent)",
};

const missingOptionalChipStyle: CSSProperties = {
  padding: "3px 10px",
  borderRadius: "var(--sy-radius-pill)",
  background: "transparent",
  color: "var(--sy-color-fg-4)",
  fontSize: "12px",
  border: "1.5px dashed var(--sy-color-line)",
};

const cliPreviewStyle: CSSProperties = {
  fontFamily: "var(--sy-font-numeric)",
  fontSize: "11.5px",
  color: "var(--sy-color-fg-4)",
  marginLeft: "auto",
  whiteSpace: "nowrap",
};

const sectionStyle: CSSProperties = {
  borderBottom: "1px solid var(--sy-color-line-soft)",
};

const sectionHeaderStyle: CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: "8px",
  padding: "8px 20px 4px",
  fontSize: "10.5px",
  fontWeight: 700,
  letterSpacing: "0.08em",
  color: "var(--sy-color-fg-4)",
  textTransform: "uppercase",
};

const sectionAnnotationStyle: CSSProperties = {
  fontWeight: 400,
  color: "var(--sy-color-fg-5)",
};

const itemStyle: CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: "10px",
  padding: "0 20px",
  height: "48px",
  cursor: "pointer",
  transition: "background var(--sy-motion-fast)",
};

const hintBarStyle: CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: "8px",
  padding: "8px 20px",
  fontSize: "11px",
  color: "var(--sy-color-fg-5)",
  borderTop: "1px solid var(--sy-color-line-soft)",
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

function relativeTime(isoStr: string): string {
  const delta = Date.now() - new Date(isoStr).getTime();
  const min = Math.floor(delta / 60_000);
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const d = Math.floor(hr / 24);
  return d === 1 ? "yesterday" : `${d} days ago`;
}
