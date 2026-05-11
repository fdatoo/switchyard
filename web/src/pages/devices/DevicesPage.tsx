import { useEffect, useState } from "react";
import { ChevronRightIcon, PluginIcon } from "@/shell/icons";

interface DriverSummary {
  name: string;
  pack: string;
  version: string;
  state: "running" | "reconnecting" | "stopped" | "unknown";
  entityCount: number;
}

async function loadDrivers(): Promise<DriverSummary[]> {
  try {
    const res = await fetch(
      "/switchyard.driver.v1.DriverManagementService/List",
      {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
          "Connect-Protocol-Version": "1",
        },
        body: "{}",
      },
    );
    if (!res.ok) return [];
    const data = (await res.json()) as {
      drivers?: Array<{
        name?: string;
        pack?: string;
        version?: string;
        state?: number;
        entity_count?: number;
        entityCount?: number;
      }>;
    };
    return (data.drivers ?? []).map((d) => ({
      name: d.name ?? "unknown",
      pack: d.pack ?? "",
      version: d.version ?? "",
      state: mapState(d.state),
      entityCount: d.entityCount ?? d.entity_count ?? 0,
    }));
  } catch {
    return [];
  }
}

function mapState(s: number | undefined): DriverSummary["state"] {
  switch (s) {
    case 1: return "running";
    case 2: return "reconnecting";
    case 3: return "stopped";
    default: return "unknown";
  }
}

const STATE_LABEL: Record<DriverSummary["state"], string> = {
  running: "Running",
  reconnecting: "Reconnecting",
  stopped: "Stopped",
  unknown: "Unknown",
};
const STATE_DOT: Record<DriverSummary["state"], string> = {
  running: "var(--sy-color-good)",
  reconnecting: "var(--sy-color-warn)",
  stopped: "var(--sy-color-fg-4)",
  unknown: "var(--sy-color-fg-5)",
};

export function DevicesPage() {
  const [drivers, setDrivers] = useState<DriverSummary[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    void loadDrivers().then((d) => {
      if (!cancelled) {
        setDrivers(d);
        setLoading(false);
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const totalEntities = drivers.reduce((sum, d) => sum + d.entityCount, 0);

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "var(--sy-space-4)",
        padding: "var(--sy-space-5) var(--sy-space-6)",
        maxWidth: "1080px",
      }}
    >
      <header>
        <h1
          style={{
            margin: 0,
            fontSize: "1.75rem",
            fontWeight: 600,
            color: "var(--sy-color-fg)",
            letterSpacing: "-0.02em",
          }}
        >
          Devices
        </h1>
        <p
          style={{
            margin: "var(--sy-space-1) 0 0",
            color: "var(--sy-color-fg-3)",
            fontSize: "0.9375rem",
          }}
        >
          {loading
            ? "Loading drivers…"
            : drivers.length === 0
            ? "No drivers installed."
            : `${drivers.length} ${drivers.length === 1 ? "driver" : "drivers"} · ${totalEntities} ${totalEntities === 1 ? "entity" : "entities"}`}
        </p>
      </header>

      {drivers.length === 0 && !loading && (
        <div
          style={{
            background: "var(--sy-color-surface-1)",
            borderRadius: "var(--sy-radius-lg)",
            boxShadow: "var(--sy-shadow)",
            padding: "var(--sy-space-5)",
            textAlign: "center",
            color: "var(--sy-color-fg-3)",
          }}
        >
          <p style={{ margin: 0 }}>
            No drivers are installed. Drivers are configured in Pkl —{" "}
            <a
              href="/_authed/settings/pkl-config"
              style={{ color: "var(--sy-color-accent)", textDecoration: "none" }}
            >
              open Pkl config
            </a>
            .
          </p>
        </div>
      )}

      {drivers.length > 0 && (
        <ul style={{ listStyle: "none", margin: 0, padding: 0 }}>
          {drivers.map((d, i) => (
            <li key={d.name}>
              <a
                href={`/_authed/devices/${encodeURIComponent(d.name)}`}
                onClick={(e) => {
                  e.preventDefault();
                  window.location.assign(
                    `/_authed/devices/${encodeURIComponent(d.name)}`,
                  );
                }}
                style={{
                  display: "grid",
                  gridTemplateColumns: "36px 1fr auto auto auto",
                  gap: "var(--sy-space-3)",
                  alignItems: "center",
                  padding: "var(--sy-space-3) var(--sy-space-4)",
                  background: "var(--sy-color-surface-1)",
                  borderRadius:
                    i === 0
                      ? "var(--sy-radius-lg) var(--sy-radius-lg) 0 0"
                      : i === drivers.length - 1
                      ? "0 0 var(--sy-radius-lg) var(--sy-radius-lg)"
                      : "0",
                  boxShadow: "var(--sy-shadow)",
                  marginTop: i === 0 ? 0 : "1px",
                  textDecoration: "none",
                  color: "var(--sy-color-fg)",
                  cursor: "pointer",
                }}
              >
                <span
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    justifyContent: "center",
                    width: "32px",
                    height: "32px",
                    borderRadius: "var(--sy-radius)",
                    background: "var(--sy-color-accent-soft)",
                    color: "var(--sy-color-accent)",
                  }}
                >
                  <PluginIcon size={16} />
                </span>
                <div style={{ minWidth: 0 }}>
                  <div
                    style={{
                      fontWeight: 600,
                      fontSize: "0.9375rem",
                      letterSpacing: "-0.005em",
                    }}
                  >
                    {d.name}
                  </div>
                  <div
                    style={{
                      fontFamily: "var(--sy-font-numeric)",
                      fontSize: "0.75rem",
                      color: "var(--sy-color-fg-4)",
                      marginTop: "2px",
                    }}
                  >
                    {d.pack || "—"} {d.version && `· ${d.version}`}
                  </div>
                </div>
                {/* Entity count + type breakdown chips */}
                <div
                  style={{
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "flex-end",
                    gap: "var(--sy-space-1)",
                  }}
                >
                  <span
                    style={{
                      fontSize: "0.75rem",
                      color: "var(--sy-color-fg-3)",
                      fontVariantNumeric: "tabular-nums",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {d.entityCount} {d.entityCount === 1 ? "entity" : "entities"}
                  </span>
                  {/* TODO: replace with real entity-type breakdown from DriverManagementService when the API provides type counts.
                      For now, assumes all entities are lights as a fallback. */}
                  {d.entityCount > 0 && (
                    <div style={{ display: "flex", gap: "4px", flexWrap: "wrap", justifyContent: "flex-end" }}>
                      <span
                        style={{
                          fontSize: "0.6875rem",
                          fontWeight: 500,
                          color: "var(--sy-color-fg-3)",
                          background: "var(--sy-color-surface-2)",
                          border: "1px solid var(--sy-color-line)",
                          padding: "1px 6px",
                          borderRadius: "var(--sy-radius-pill)",
                          whiteSpace: "nowrap",
                        }}
                      >
                        {d.entityCount} {d.entityCount === 1 ? "light" : "lights"}
                      </span>
                    </div>
                  )}
                </div>
                <span
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: "var(--sy-space-1)",
                    fontSize: "0.8125rem",
                    color: "var(--sy-color-fg-3)",
                  }}
                >
                  <span
                    aria-hidden="true"
                    style={{
                      width: "8px",
                      height: "8px",
                      borderRadius: "var(--sy-radius-pill)",
                      background: STATE_DOT[d.state],
                    }}
                  />
                  {STATE_LABEL[d.state]}
                </span>
                <ChevronRightIcon size={14} />
              </a>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
