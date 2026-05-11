import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { RoomsGridSection } from "./RoomsGridSection";
import type { RoomSummary } from "./hooks/useHomeRooms";

const makeMockRooms = (count: number): RoomSummary[] =>
  Array.from({ length: count }, (_, i) => ({
    id: `room-${i}`,
    name: i === 0 ? "Kitchen" : `Room ${i + 1}`,
    entityCount: i === 0 ? "3 lights" : "2 lights",
    scenes: ["Scene A", "Scene B"],
    statePill: "On",
  }));

describe("RoomsGridSection", () => {
  it("renders up to 8 room tiles when given 10 rooms", () => {
    render(<RoomsGridSection rooms={makeMockRooms(10)} />);

    const tiles = screen.getAllByTestId("room-tile");
    expect(tiles).toHaveLength(8);
  });

  it('renders a "View all" link pointing to /rooms', () => {
    render(<RoomsGridSection rooms={makeMockRooms(3)} />);

    const link = screen.getByRole("link", { name: /view all/i });
    expect(link.getAttribute("href")).toBe("/rooms");
  });

  it("each tile shows room name and entity count", () => {
    render(<RoomsGridSection rooms={makeMockRooms(3)} />);

    expect(screen.getByText("Kitchen")).toBeDefined();
    expect(screen.getByText("3 lights")).toBeDefined();
  });

  it("shows at most 3 scene chips per tile", () => {
    const roomsWithManyScenes: RoomSummary[] = [
      {
        id: "r1",
        name: "Kitchen",
        entityCount: "3 lights",
        scenes: ["A", "B", "C", "D", "E"],
        statePill: "On",
      },
    ];
    render(<RoomsGridSection rooms={roomsWithManyScenes} />);

    // Only first 3 scenes shown
    expect(screen.getByText("A")).toBeDefined();
    expect(screen.getByText("B")).toBeDefined();
    expect(screen.getByText("C")).toBeDefined();
    expect(screen.queryByText("D")).toBeNull();
    expect(screen.queryByText("E")).toBeNull();
  });
});
