/**
 * RoomTile.tsx — room name, icon, entity-count badge + fidelity slot props.
 *
 * Fidelity props (Plan 07):
 *   width   — "standard" (default) | "wide"  → grid-column: span 2
 *   scenes  — 0 | 2 | 4  → number of inline scene chips
 *   metric  — "none" | "sensor" | "presence" | "now_playing" | "next_automation" | "last_activity"
 *
 * Alert dimming: if any entity ID of this tile appears in the AlertContext's
 * affectedEntityIds, the tile renders at opacity 0.55 with a "stale" badge.
 */

import { registerTile } from "../../registry";
import type { TileProps } from "../../registry";
import { useAlertState } from "@/ambient/AlertPill";

// ---------------------------------------------------------------------------
// Fidelity types
// ---------------------------------------------------------------------------

export type TileWidth  = "standard" | "wide";
export type TileScenes = 0 | 2 | 4;
export type TileMetric =
  | "none"
  | "sensor"
  | "presence"
  | "now_playing"
  | "next_automation"
  | "last_activity";

export interface RoomTileFidelity {
  width?: TileWidth;
  scenes?: TileScenes;
  metric?: TileMetric;
}

// ---------------------------------------------------------------------------
// Metric row renderers
// ---------------------------------------------------------------------------

const METRIC_LABELS: Record<TileMetric, string> = {
  none:            "",
  sensor:          "Sensor",
  presence:        "Presence",
  now_playing:     "Now Playing",
  next_automation: "Next Automation",
  last_activity:   "Last Activity",
};

function MetricRow({ metric }: { metric: TileMetric }) {
  if (metric === "none") return null;
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: "var(--sy-space-2)",
        marginTop: "var(--sy-space-2)",
        fontSize: "0.75rem",
        color: "var(--sy-color-fg-3)",
        borderTop: "1px solid var(--sy-color-line-soft)",
        paddingTop: "var(--sy-space-2)",
      }}
    >
      <span>{METRIC_LABELS[metric]}</span>
      <span style={{ color: "var(--sy-color-fg-4)" }}>—</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Scene chips
// ---------------------------------------------------------------------------

function SceneStrip({ count, isAffected }: {
  count: TileScenes;
  isAffected: boolean;
}) {
  if (count === 0) return null;

  return (
    <div style={{ display: "flex", gap: "var(--sy-space-1)", flexWrap: "wrap", marginTop: "var(--sy-space-2)" }}>
      {Array.from({ length: count }, (_, i) => (
        <span
          key={i}
          style={{
            background: "var(--sy-color-surface-1)",
            borderRadius: "var(--sy-radius-pill)",
            fontSize: "0.625rem",
            fontWeight: 500,
            padding: "0.125rem 0.5rem",
            color: isAffected ? "var(--sy-color-fg-4)" : "var(--sy-color-fg-3)",
            border: "1px solid var(--sy-color-line-soft)",
            cursor: "pointer",
          }}
        >
          Scene {i + 1}
        </span>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Safe useAlertState — does not throw when used outside AlertContext
// ---------------------------------------------------------------------------

function useSafeAlertState(): string[] {
  try {
    const ctx = useAlertState();
    return ctx.alertState.affectedEntityIds;
  } catch {
    return [];
  }
}

// ---------------------------------------------------------------------------
// RoomTile component
// ---------------------------------------------------------------------------

function RoomTile({ def }: TileProps) {
  const label       = (def.props.label as string) ?? def.props.roomSlug ?? "Room";
  const entityCount = (def.props.entityCount as number) ?? 0;

  // Fidelity props with defaults from plan spec
  const width:    TileWidth  = (def.props.width    as TileWidth)  ?? "standard";
  const scenes:   TileScenes = (def.props.scenes   as TileScenes) ?? 2;
  const metric:   TileMetric = (def.props.metric   as TileMetric) ?? "sensor";
  const entityIds: string[]  = (def.props.entityIds as string[])  ?? [];

  // Alert context — may not be present (outside AmbientRoot)
  const affectedEntityIds = useSafeAlertState();
  const isAffected = entityIds.length > 0 && entityIds.some((id) => affectedEntityIds.includes(id));

  return (
    <div
      data-testid={`room-tile-${String(label)}`}
      data-fidelity-width={width}
      data-fidelity-scenes={scenes}
      data-fidelity-metric={metric}
      style={{
        background: "var(--sy-color-surface-2)",
        borderRadius: "var(--sy-radius)",
        padding: "var(--sy-space-3)",
        display: "flex",
        flexDirection: "column",
        gap: "var(--sy-space-2)",
        minHeight: "100px",
        position: "relative",
        cursor: "pointer",
        transition: "background var(--sy-motion-fast), opacity var(--sy-motion)",
        // Fidelity: width
        ...(width === "wide" ? { gridColumn: "span 2" } : {}),
        // Alert dimming
        opacity: isAffected ? 0.55 : 1,
      }}
    >
      {/* Room icon */}
      <div
        style={{
          width: "2rem",
          height: "2rem",
          background: "var(--sy-color-accent-soft)",
          borderRadius: "var(--sy-radius-sm)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontSize: "1.25rem",
        }}
      >
        🏠
      </div>

      {/* Room name + stale badge */}
      <div style={{ display: "flex", alignItems: "center", gap: "var(--sy-space-2)" }}>
        <span
          style={{
            fontSize: "0.875rem",
            fontWeight: 600,
            color: "var(--sy-color-fg)",
          }}
        >
          {String(label)}
        </span>
        {isAffected && (
          <span
            data-testid="stale-badge"
            style={{
              background: "color-mix(in srgb, var(--sy-color-warn) 20%, transparent)",
              color: "var(--sy-color-warn)",
              borderRadius: "var(--sy-radius-pill)",
              fontSize: "0.5625rem",
              fontWeight: 600,
              padding: "0.1rem 0.375rem",
            }}
          >
            stale
          </span>
        )}
      </div>

      {/* Entity count badge */}
      {entityCount > 0 && (
        <span
          style={{
            position: "absolute",
            top: "var(--sy-space-2)",
            right: "var(--sy-space-2)",
            background: "var(--sy-color-accent)",
            color: "var(--sy-color-bg)",
            borderRadius: "var(--sy-radius-pill)",
            fontSize: "0.625rem",
            fontWeight: 700,
            padding: "0.125rem 0.375rem",
          }}
        >
          {entityCount}
        </span>
      )}

      {/* Scene chips strip */}
      <SceneStrip count={scenes} isAffected={isAffected} />

      {/* Metric row */}
      <MetricRow metric={metric} />
    </div>
  );
}

registerTile("RoomTile", RoomTile);

export { RoomTile };
