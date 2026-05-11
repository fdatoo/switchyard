import type { DriverSummary } from "@/data/driver-management-client";
import { Button } from "@/theme/primitives/button";

export const SCOPE_TOOLTIP = "Requires the settings.drivers.write scope";

interface StatTileProps {
  label: string;
  value: string | number;
}

function StatTile({ label, value }: StatTileProps) {
  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "var(--sy-space-1)",
        padding: "var(--sy-space-3) var(--sy-space-4)",
        background: "var(--sy-color-surface-2)",
        borderRadius: "var(--sy-radius)",
        minWidth: "120px",
        flex: 1,
      }}
    >
      <span
        style={{
          fontSize: "0.6875rem",
          fontWeight: 600,
          textTransform: "uppercase",
          letterSpacing: "0.06em",
          color: "var(--sy-color-fg-4)",
        }}
      >
        {label}
      </span>
      <span
        style={{
          fontFamily: "var(--sy-font-numeric)",
          fontSize: "1.125rem",
          fontWeight: 600,
          color: "var(--sy-color-fg)",
        }}
      >
        {value}
      </span>
    </div>
  );
}

interface IdentityRowProps {
  label: string;
  value: string;
}

function IdentityRow({ label, value }: IdentityRowProps) {
  return (
    <tr>
      <td
        style={{
          padding: "var(--sy-space-1) var(--sy-space-3) var(--sy-space-1) 0",
          fontSize: "0.75rem",
          fontWeight: 500,
          color: "var(--sy-color-fg-4)",
          whiteSpace: "nowrap",
          verticalAlign: "top",
        }}
      >
        {label}
      </td>
      <td
        style={{
          padding: "var(--sy-space-1) 0",
          fontSize: "0.8125rem",
          color: "var(--sy-color-fg)",
          fontFamily: value.startsWith("/") || value.startsWith("0x") ? "var(--sy-font-numeric)" : undefined,
          wordBreak: "break-all",
        }}
      >
        {value || "—"}
      </td>
    </tr>
  );
}

interface ExpandedDetailProps {
  driver: DriverSummary;
  logLines: string[];
  hasWriteScope: boolean;
  onStop: () => void;
  onRestart: () => void;
}

export function ExpandedDetail({
  driver,
  logLines,
  hasWriteScope,
  onStop,
  onRestart,
}: ExpandedDetailProps) {
  return (
    <div
      style={{
        padding: "var(--sy-space-4)",
        background: "var(--sy-color-surface-1)",
        borderTop: "1px solid var(--sy-color-line)",
        display: "flex",
        flexDirection: "column",
        gap: "var(--sy-space-4)",
      }}
    >
      {/* Identity table */}
      <div>
        <p
          style={{
            margin: "0 0 var(--sy-space-2)",
            fontSize: "0.6875rem",
            fontWeight: 600,
            textTransform: "uppercase",
            letterSpacing: "0.06em",
            color: "var(--sy-color-fg-4)",
          }}
        >
          Identity
        </p>
        <table style={{ borderCollapse: "collapse", width: "100%" }}>
          <tbody>
            <IdentityRow label="Pack" value={driver.pack} />
            <IdentityRow label="Version" value={driver.version} />
            <IdentityRow label="PID" value={driver.pid > 0 ? String(driver.pid) : "—"} />
            <IdentityRow label="Socket" value={driver.socket} />
            <IdentityRow label="Config" value={driver.configFile} />
            <IdentityRow label="OTel span" value={driver.otelSpan} />
          </tbody>
        </table>
      </div>

      {/* Metrics row */}
      <div>
        <p
          style={{
            margin: "0 0 var(--sy-space-2)",
            fontSize: "0.6875rem",
            fontWeight: 600,
            textTransform: "uppercase",
            letterSpacing: "0.06em",
            color: "var(--sy-color-fg-4)",
          }}
        >
          Metrics
        </p>
        <div style={{ display: "flex", gap: "var(--sy-space-3)", flexWrap: "wrap" }}>
          <StatTile label="Entities" value={driver.entityCount} />
          <StatTile label="Events/day" value={driver.eventsPerDay.toFixed(0)} />
          <StatTile label="Last cmd ack" value={`${driver.lastCmdAckMs}ms`} />
          <StatTile label="Reconnects today" value={driver.reconnectsToday} />
        </div>
      </div>

      {/* Recent logs */}
      <div>
        <p
          style={{
            margin: "0 0 var(--sy-space-2)",
            fontSize: "0.6875rem",
            fontWeight: 600,
            textTransform: "uppercase",
            letterSpacing: "0.06em",
            color: "var(--sy-color-fg-4)",
          }}
        >
          Recent logs
        </p>
        <pre
          style={{
            margin: 0,
            padding: "var(--sy-space-3)",
            background: "#0d1117",
            color: "#c9d1d9",
            borderRadius: "var(--sy-radius)",
            fontFamily: "var(--sy-font-numeric)",
            fontSize: "0.75rem",
            lineHeight: 1.6,
            overflowX: "auto",
            whiteSpace: "pre-wrap",
            wordBreak: "break-all",
          }}
        >
          {logLines.length > 0
            ? logLines.join("\n")
            : "(no log lines available)"}
        </pre>
      </div>

      {/* Actions */}
      <div style={{ display: "flex", gap: "var(--sy-space-3)", flexWrap: "wrap" }}>
        <a
          href={`/activity?driver=${driver.id}`}
          style={{
            display: "inline-flex",
            alignItems: "center",
            padding: "var(--sy-space-2) var(--sy-space-4)",
            borderRadius: "var(--sy-radius)",
            background: "var(--sy-color-surface-2)",
            color: "var(--sy-color-fg)",
            border: "1px solid var(--sy-color-line)",
            fontSize: "0.875rem",
            fontWeight: 500,
            textDecoration: "none",
            cursor: "pointer",
          }}
        >
          Open in Time-machine
        </a>
        <a
          href={`/devices?driver=${driver.id}`}
          style={{
            display: "inline-flex",
            alignItems: "center",
            padding: "var(--sy-space-2) var(--sy-space-4)",
            borderRadius: "var(--sy-radius)",
            background: "var(--sy-color-surface-2)",
            color: "var(--sy-color-fg)",
            border: "1px solid var(--sy-color-line)",
            fontSize: "0.875rem",
            fontWeight: 500,
            textDecoration: "none",
            cursor: "pointer",
          }}
        >
          Inspect entities
        </a>
        <Button
          variant="secondary"
          onClick={onStop}
          disabled={!hasWriteScope}
          title={!hasWriteScope ? SCOPE_TOOLTIP : undefined}
        >
          Stop driver
        </Button>
        <Button
          variant="secondary"
          onClick={onRestart}
          disabled={!hasWriteScope}
          title={!hasWriteScope ? SCOPE_TOOLTIP : undefined}
        >
          Restart
        </Button>
      </div>
    </div>
  );
}
