import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { RoomTile } from "./RoomTile";
import type { TileDef } from "../../model";

// Helper to create a TileDef
function makeDef(props: Record<string, unknown>): { def: TileDef } {
  return {
    def: {
      id: "tile-1",
      type: "RoomTile",
      props,
    },
  };
}

describe("RoomTile — fidelity slot props", () => {
  it("minimal preset: scenes=0, metric=none", () => {
    render(<RoomTile {...makeDef({ label: "Kitchen", scenes: 0, metric: "none", width: "standard" })} />);
    const tile = screen.getByTestId("room-tile-Kitchen");
    expect(tile).toBeInTheDocument();
    expect(tile).toHaveAttribute("data-fidelity-scenes", "0");
    expect(tile).toHaveAttribute("data-fidelity-metric", "none");
    expect(tile).toHaveAttribute("data-fidelity-width", "standard");
    // No scene chips
    expect(screen.queryByText("Scene 1")).toBeNull();
    // No metric row
    expect(screen.queryByText("Sensor")).toBeNull();
  });

  it("balanced preset: scenes=2, metric=sensor, width=standard", () => {
    render(<RoomTile {...makeDef({ label: "Living Room", scenes: 2, metric: "sensor", width: "standard" })} />);
    const tile = screen.getByTestId("room-tile-Living Room");
    expect(tile).toHaveAttribute("data-fidelity-scenes", "2");
    expect(tile).toHaveAttribute("data-fidelity-metric", "sensor");
    expect(tile).toHaveAttribute("data-fidelity-width", "standard");
    // 2 scene chips
    expect(screen.getByText("Scene 1")).toBeInTheDocument();
    expect(screen.getByText("Scene 2")).toBeInTheDocument();
    expect(screen.queryByText("Scene 3")).toBeNull();
    // Metric row
    expect(screen.getByText("Sensor")).toBeInTheDocument();
  });

  it("rich preset: scenes=4, metric=sensor, width=wide", () => {
    render(<RoomTile {...makeDef({ label: "Bedroom", scenes: 4, metric: "now_playing", width: "wide" })} />);
    const tile = screen.getByTestId("room-tile-Bedroom");
    expect(tile).toHaveAttribute("data-fidelity-scenes", "4");
    expect(tile).toHaveAttribute("data-fidelity-metric", "now_playing");
    expect(tile).toHaveAttribute("data-fidelity-width", "wide");
    // 4 scene chips
    expect(screen.getByText("Scene 1")).toBeInTheDocument();
    expect(screen.getByText("Scene 4")).toBeInTheDocument();
    // Metric row
    expect(screen.getByText("Now Playing")).toBeInTheDocument();
  });

  it("defaults: width=standard, scenes=2, metric=sensor when not specified", () => {
    render(<RoomTile {...makeDef({ label: "Hall" })} />);
    const tile = screen.getByTestId("room-tile-Hall");
    expect(tile).toHaveAttribute("data-fidelity-width", "standard");
    expect(tile).toHaveAttribute("data-fidelity-scenes", "2");
    expect(tile).toHaveAttribute("data-fidelity-metric", "sensor");
  });

  it("renders room name", () => {
    render(<RoomTile {...makeDef({ label: "Office" })} />);
    expect(screen.getByText("Office")).toBeInTheDocument();
  });

  it("renders entity count badge when entityCount > 0", () => {
    render(<RoomTile {...makeDef({ label: "Garden", entityCount: 5 })} />);
    expect(screen.getByText("5")).toBeInTheDocument();
  });

  it("does not render entity count badge when entityCount = 0", () => {
    render(<RoomTile {...makeDef({ label: "Attic", entityCount: 0 })} />);
    expect(screen.queryByText("0")).toBeNull();
  });
});
