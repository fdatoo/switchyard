import { useHomeRightNow, type StatTile } from "./hooks/useHomeRightNow";

interface RightNowStripSectionProps {
  /** Allow passing mock tiles for testing */
  tiles?: StatTile[];
}

/**
 * RightNowStripSection — renders four stat tiles in a horizontal flex row.
 * Each tile shows a label, large value+unit, and a sublabel.
 */
export function RightNowStripSection({ tiles }: RightNowStripSectionProps) {
  const defaultTiles = useHomeRightNow();
  const statTiles = tiles ?? defaultTiles;

  return (
    <div
      style={{
        display: "flex",
        gap: "var(--sy-space-3)",
        flexWrap: "wrap",
      }}
    >
      {statTiles.map((tile) => (
        <div
          key={tile.id}
          data-testid="stat-tile"
          style={{
            flex: "1 1 0",
            minWidth: "120px",
            background: "var(--sy-color-surface-1)",
            border: "1px solid var(--sy-color-line)",
            borderRadius: "var(--sy-radius-lg)",
            padding: "var(--sy-space-4)",
            display: "flex",
            flexDirection: "column",
            gap: "var(--sy-space-1)",
          }}
        >
          <span
            style={{
              fontSize: "0.6875rem",
              fontWeight: 500,
              color: "var(--sy-color-fg-4)",
              textTransform: "uppercase",
              letterSpacing: "0.06em",
            }}
          >
            {tile.label}
          </span>
          <div
            style={{
              display: "flex",
              alignItems: "baseline",
              gap: "var(--sy-space-1)",
            }}
          >
            <span
              style={{
                fontFamily: "var(--sy-font-numeric)",
                fontSize: "1.75rem",
                fontWeight: 600,
                color: "var(--sy-color-fg)",
                lineHeight: 1,
              }}
            >
              {tile.value}
            </span>
            <span
              style={{
                fontSize: "0.8125rem",
                fontWeight: 400,
                color: "var(--sy-color-fg-3)",
              }}
            >
              {tile.unit}
            </span>
          </div>
          <span
            style={{
              fontSize: "0.6875rem",
              color: "var(--sy-color-fg-4)",
            }}
          >
            {tile.sublabel}
          </span>
        </div>
      ))}
    </div>
  );
}
