import { useState } from "react";
import type { DriverSummary } from "@/data/driver-management-client";
import { driverClient } from "@/data/driver-management-client";
import { Chip } from "@/theme/primitives/chip";
import { ExpandedDetail } from "./ExpandedDetail";

function statusColor(status: string): string {
  switch (status) {
    case "healthy":
      return "var(--sy-color-good)";
    case "reconnecting":
      return "var(--sy-color-warn)";
    case "degraded":
      return "var(--sy-color-bad)";
    default:
      return "var(--sy-color-fg-4)";
  }
}

function uptimeLabel(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  return m > 0 ? `${h}h ${m}m` : `${h}h`;
}

export interface DriverCardProps {
  driver: DriverSummary;
  expanded: boolean;
  onToggle: (id: string) => void;
  hasWriteScope: boolean;
}

export function DriverCard({ driver, expanded, onToggle, hasWriteScope }: DriverCardProps) {
  const [logLines, setLogLines] = useState<string[]>([]);
  const [loadingLogs, setLoadingLogs] = useState(false);

  const handleToggle = async () => {
    if (!expanded) {
      setLoadingLogs(true);
      try {
        const lines = await driverClient.logs(driver.id, 8);
        setLogLines(lines);
      } catch {
        setLogLines(["(could not load logs)"]);
      } finally {
        setLoadingLogs(false);
      }
    }
    onToggle(driver.id);
  };

  return (
    <div
      style={{
        borderRadius: "var(--sy-radius)",
        border: "1px solid var(--sy-color-line)",
        overflow: "hidden",
        marginBottom: "var(--sy-space-2)",
      }}
    >
      {/* Collapsed row */}
      <button
        onClick={() => void handleToggle()}
        aria-expanded={expanded}
        style={{
          display: "flex",
          alignItems: "center",
          width: "100%",
          padding: "var(--sy-space-3) var(--sy-space-4)",
          background: expanded ? "var(--sy-color-surface-1)" : "var(--sy-color-bg)",
          border: "none",
          cursor: "pointer",
          gap: "var(--sy-space-4)",
          textAlign: "left",
        }}
      >
        <div style={{ flex: 1, minWidth: 0 }}>
          <div
            style={{
              fontWeight: 600,
              fontSize: "0.875rem",
              color: "var(--sy-color-fg)",
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}
          >
            {driver.pack}
          </div>
          <div
            style={{
              fontSize: "0.75rem",
              color: "var(--sy-color-fg-4)",
            }}
          >
            v{driver.version} · {uptimeLabel(driver.uptimeSeconds)} uptime
          </div>
        </div>

        {/* Status chip */}
        <Chip
          style={{
            background: "transparent",
            border: `1px solid ${statusColor(driver.status)}`,
            color: statusColor(driver.status),
          }}
        >
          {driver.status}
        </Chip>

        {/* Expand indicator */}
        <span
          style={{
            color: "var(--sy-color-fg-4)",
            fontSize: "0.75rem",
            transition: "var(--sy-motion-fast)",
            transform: expanded ? "rotate(180deg)" : "rotate(0)",
            display: "inline-block",
          }}
        >
          ▾
        </span>
      </button>

      {/* Expanded card */}
      {expanded && (
        loadingLogs ? (
          <div
            style={{
              padding: "var(--sy-space-4)",
              color: "var(--sy-color-fg-4)",
              fontSize: "0.8125rem",
              borderTop: "1px solid var(--sy-color-line)",
            }}
          >
            Loading…
          </div>
        ) : (
          <ExpandedDetail
            driver={driver}
            logLines={logLines}
            hasWriteScope={hasWriteScope}
            onStop={() => void driverClient.stop(driver.id, "user requested stop")}
            onRestart={() => void driverClient.restart(driver.id, "user requested restart")}
          />
        )
      )}
    </div>
  );
}
