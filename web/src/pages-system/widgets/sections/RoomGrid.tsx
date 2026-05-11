/**
 * RoomGrid.tsx — room grid section with TileHost for RoomTile children.
 * In developer language, renders as a sortable RoomsTable instead.
 */

import { registerSection } from "../../registry";
import type { SectionProps } from "../../registry";
import { TileHost } from "../../render/TileHost";
import type { TileDef } from "../../model";
import { useLanguage } from "../../../theme/language-provider";
import { RoomsTable, type RoomRow } from "./RoomsTable";

function RoomGridSection({ def, editMode }: SectionProps) {
  const { language } = useLanguage();
  const title = def.props.title as string | undefined;
  const tiles = (def.tiles ?? []) as TileDef[];

  // In developer language, render rooms as a sortable table
  if (language === "developer") {
    const tableRows: RoomRow[] = tiles.map((tile) => ({
      id: tile.id,
      name: (tile.props.name as string | undefined) ?? tile.id,
      state: ((tile.props.state as string | undefined) === "on" ? "on" : "off") as "on" | "off",
      scene: (tile.props.scene as string | undefined) ?? "—",
      brightness: (tile.props.brightness as number | undefined) ?? 0,
      sinceMs: (tile.props.sinceMs as number | undefined) ?? 0,
    }));

    return (
      <div>
        {title && (
          <div
            style={{
              padding: "var(--sy-space-3) var(--sy-space-4)",
              borderBottom: "1px solid var(--sy-color-line-soft)",
            }}
          >
            <h3
              style={{
                margin: 0,
                fontSize: "0.875rem",
                fontWeight: 600,
                fontFamily: "var(--sy-font-numeric)",
                color: "var(--sy-color-fg)",
                letterSpacing: "0.02em",
              }}
            >
              {title}
            </h3>
          </div>
        )}
        <RoomsTable rooms={tableRows} />
      </div>
    );
  }

  return (
    <div>
      {title && (
        <div
          style={{
            padding: "var(--sy-space-3) var(--sy-space-4)",
            borderBottom: "1px solid var(--sy-color-line-soft)",
          }}
        >
          <h3
            style={{
              margin: 0,
              fontSize: "1rem",
              fontWeight: 600,
              color: "var(--sy-color-fg)",
            }}
          >
            {title}
          </h3>
        </div>
      )}
      <TileHost tiles={tiles} editMode={editMode} />
    </div>
  );
}

registerSection("RoomGrid", RoomGridSection);

export { RoomGridSection };
