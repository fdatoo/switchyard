import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { RightNowStripSection } from "./RightNowStripSection";
import type { StatTile } from "./hooks/useHomeRightNow";

const mockTiles: StatTile[] = [
  { id: "temp", label: "Indoor temp", value: "21.4", unit: "°C", sublabel: "Living room" },
  { id: "co2", label: "Office CO₂", value: "812", unit: "ppm", sublabel: "Good" },
  { id: "lights", label: "Lights on", value: "6", unit: "/ 23", sublabel: "6 rooms active" },
  { id: "events", label: "Events/min", value: "128", unit: "avg", sublabel: "Last 5 min" },
];

describe("RightNowStripSection", () => {
  it("renders four stat tiles", () => {
    render(<RightNowStripSection tiles={mockTiles} />);

    const tiles = screen.getAllByTestId("stat-tile");
    expect(tiles).toHaveLength(4);
  });

  it("displays correct labels and values", () => {
    render(<RightNowStripSection tiles={mockTiles} />);

    expect(screen.getByText("Indoor temp")).toBeDefined();
    expect(screen.getByText("21.4")).toBeDefined();
    expect(screen.getByText("Office CO₂")).toBeDefined();
  });

  it("renders with default hook data when no tiles prop given", () => {
    render(<RightNowStripSection />);

    const tiles = screen.getAllByTestId("stat-tile");
    expect(tiles).toHaveLength(4);
  });
});
