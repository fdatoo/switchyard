import { useHomeRooms, type RoomSummary } from "./hooks/useHomeRooms";

interface RoomsGridSectionProps {
  /** Allow passing mock rooms for testing */
  rooms?: RoomSummary[];
}

/**
 * RoomsGridSection — renders up to 8 room tiles in a 3-column CSS grid.
 * Each tile shows room name, entity count, up to 3 scene chips, and state pill.
 * Uses Chip and Pill styling via --sy-* tokens directly (no context needed for layout).
 */
export function RoomsGridSection({ rooms }: RoomsGridSectionProps) {
  const defaultRooms = useHomeRooms();
  const allRooms = rooms ?? defaultRooms;
  // Decision #5: show up to 8 rooms only
  const displayRooms = allRooms.slice(0, 8);

  return (
    <section>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          marginBottom: "var(--sy-space-3)",
        }}
      >
        <h2
          style={{
            margin: 0,
            fontSize: "0.8125rem",
            fontWeight: 600,
            letterSpacing: "0.06em",
            textTransform: "uppercase",
            color: "var(--sy-color-fg-4)",
          }}
        >
          Rooms
        </h2>
        <a
          href="/rooms"
          style={{
            fontSize: "0.8125rem",
            color: "var(--sy-color-accent)",
            textDecoration: "none",
            fontWeight: 500,
          }}
        >
          View all →
        </a>
      </div>
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(3, 1fr)",
          gap: "var(--sy-space-3)",
        }}
      >
        {displayRooms.map((room) => (
          <RoomTile key={room.id} room={room} />
        ))}
      </div>
    </section>
  );
}

interface RoomTileProps {
  room: RoomSummary;
}

function RoomTile({ room }: RoomTileProps) {
  const visibleScenes = room.scenes.slice(0, 3);

  return (
    <div
      data-testid="room-tile"
      style={{
        background: "var(--sy-color-surface-1)",
        border: "1px solid var(--sy-color-line)",
        borderRadius: "var(--sy-radius-lg)",
        padding: "var(--sy-space-4)",
        display: "flex",
        flexDirection: "column",
        gap: "var(--sy-space-2)",
      }}
    >
      {/* Header row: room name + state pill */}
      <div
        style={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          gap: "var(--sy-space-2)",
        }}
      >
        <div>
          <div
            style={{
              fontSize: "0.9375rem",
              fontWeight: 600,
              color: "var(--sy-color-fg)",
              lineHeight: 1.3,
            }}
          >
            {room.name}
          </div>
          <div
            style={{
              fontSize: "0.75rem",
              color: "var(--sy-color-fg-4)",
              marginTop: "1px",
            }}
          >
            {room.entityCount}
          </div>
        </div>
        {/* State pill */}
        <span
          style={{
            display: "inline-flex",
            alignItems: "center",
            padding: "1px var(--sy-space-2)",
            borderRadius: "var(--sy-radius-pill)",
            fontSize: "0.6875rem",
            fontWeight: 500,
            background: "var(--sy-color-surface-2)",
            color: "var(--sy-color-fg-3)",
            flexShrink: 0,
          }}
        >
          {room.statePill}
        </span>
      </div>
      {/* Scene chips */}
      {visibleScenes.length > 0 && (
        <div
          style={{
            display: "flex",
            flexWrap: "wrap",
            gap: "var(--sy-space-1)",
          }}
        >
          {visibleScenes.map((scene) => (
            <span
              key={scene}
              style={{
                display: "inline-flex",
                alignItems: "center",
                padding: "var(--sy-space-1) var(--sy-space-2)",
                borderRadius: "var(--sy-radius-pill)",
                fontSize: "0.6875rem",
                fontWeight: 500,
                background: "var(--sy-color-surface-2)",
                color: "var(--sy-color-fg-3)",
                border: "1px solid var(--sy-color-line)",
              }}
            >
              {scene}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
