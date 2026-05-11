import { useState } from "react";
import { useInstalledPacks } from "@/data/widget-pack-client";
import type { InstalledPack, SignatureStatus } from "@/data/widget-pack-client";
import { Button } from "@/theme/primitives/button";
import { Chip } from "@/theme/primitives/chip";
import { Surface } from "@/theme/primitives/surface";

function signatureColor(status: SignatureStatus): string {
  switch (status) {
    case "verified":
      return "var(--sy-color-good)";
    case "unverified":
      return "var(--sy-color-warn)";
    default:
      return "var(--sy-color-fg-4)";
  }
}

interface PackRowProps {
  pack: InstalledPack;
}

function PackRow({ pack }: PackRowProps) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: "var(--sy-space-4)",
        padding: "var(--sy-space-3) 0",
        borderBottom: "1px solid var(--sy-color-line)",
      }}
    >
      <div style={{ flex: 1, minWidth: 0 }}>
        <div
          style={{
            fontFamily: "var(--sy-font-numeric)",
            fontSize: "0.8125rem",
            color: "var(--sy-color-fg)",
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {pack.ociRef}
        </div>
        <div style={{ fontSize: "0.75rem", color: "var(--sy-color-fg-4)" }}>
          v{pack.version}
        </div>
      </div>
      <Chip
        style={{
          background: "transparent",
          border: `1px solid ${signatureColor(pack.signature)}`,
          color: signatureColor(pack.signature),
          flexShrink: 0,
        }}
      >
        {pack.signature}
      </Chip>
    </div>
  );
}

interface InstallDialogProps {
  onInstall: (ociRef: string) => Promise<void>;
  onClose: () => void;
}

function InstallDialog({ onInstall, onClose }: InstallDialogProps) {
  const [ociRef, setOciRef] = useState("");
  const [installing, setInstalling] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const handleInstall = async () => {
    if (!ociRef.trim()) return;
    setInstalling(true);
    setErr(null);
    try {
      await onInstall(ociRef.trim());
      onClose();
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "Install failed");
    } finally {
      setInstalling(false);
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Install widget pack"
      style={{
        position: "fixed",
        inset: 0,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: "var(--sy-color-overlay)",
        zIndex: 1000,
      }}
    >
      <Surface
        style={{
          padding: "var(--sy-space-5)",
          minWidth: "360px",
          border: "1px solid var(--sy-color-line)",
        }}
      >
        <h2
          style={{
            margin: "0 0 var(--sy-space-4)",
            fontSize: "1rem",
            fontWeight: 600,
            color: "var(--sy-color-fg)",
          }}
        >
          Install widget pack
        </h2>
        <input
          type="text"
          value={ociRef}
          onChange={(e) => setOciRef(e.target.value)}
          placeholder="ghcr.io/owner/pack:version"
          style={{
            width: "100%",
            padding: "var(--sy-space-2) var(--sy-space-3)",
            border: "1px solid var(--sy-color-line)",
            borderRadius: "var(--sy-radius)",
            fontSize: "0.875rem",
            fontFamily: "var(--sy-font-numeric)",
            background: "var(--sy-color-bg)",
            color: "var(--sy-color-fg)",
            marginBottom: "var(--sy-space-3)",
            boxSizing: "border-box",
          }}
        />
        {err && (
          <p
            style={{
              color: "var(--sy-color-bad)",
              fontSize: "0.8125rem",
              margin: "0 0 var(--sy-space-3)",
            }}
          >
            {err}
          </p>
        )}
        <div style={{ display: "flex", gap: "var(--sy-space-3)", justifyContent: "flex-end" }}>
          <Button variant="ghost" onClick={onClose} disabled={installing}>
            Cancel
          </Button>
          <Button
            variant="primary"
            onClick={() => void handleInstall()}
            disabled={installing || !ociRef.trim()}
          >
            {installing ? "Installing…" : "Install"}
          </Button>
        </div>
      </Surface>
    </div>
  );
}

/**
 * WidgetPacks section — shows installed packs with signature status chips,
 * and an install dialog for adding new packs via OCI ref.
 */
export function WidgetPacks() {
  const { packs, loading, error, installPack } = useInstalledPacks();
  const [dialogOpen, setDialogOpen] = useState(false);

  return (
    <div>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          marginBottom: "var(--sy-space-5)",
        }}
      >
        <h1
          style={{
            margin: 0,
            fontSize: "1.25rem",
            fontWeight: 600,
            color: "var(--sy-color-fg)",
          }}
        >
          Widget packs
        </h1>
        <Button variant="secondary" onClick={() => setDialogOpen(true)}>
          + Install
        </Button>
      </div>

      {loading && (
        <p style={{ color: "var(--sy-color-fg-4)", fontStyle: "italic" }}>
          Loading packs…
        </p>
      )}
      {error && (
        <p style={{ color: "var(--sy-color-bad)" }}>Error: {error}</p>
      )}
      {!loading && !error && (
        <Surface
          style={{
            padding: "0 var(--sy-space-4)",
            border: "1px solid var(--sy-color-line)",
          }}
        >
          {packs.length === 0 ? (
            <p
              style={{
                color: "var(--sy-color-fg-4)",
                fontStyle: "italic",
                fontSize: "0.8125rem",
                padding: "var(--sy-space-4) 0",
              }}
            >
              No widget packs installed.
            </p>
          ) : (
            packs.map((pack: InstalledPack) => (
              <PackRow key={`${pack.name}@${pack.version}`} pack={pack} />
            ))
          )}
        </Surface>
      )}

      {dialogOpen && (
        <InstallDialog
          onInstall={installPack}
          onClose={() => setDialogOpen(false)}
        />
      )}
    </div>
  );
}
