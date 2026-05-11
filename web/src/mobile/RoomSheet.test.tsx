import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RoomSheet } from "./RoomSheet";

const fakeRoom = {
  slug: "living-room",
  name: "Living Room",
  brightness: 75,
  scenes: ["Movie", "Dinner", "Bright", "Night"],
  entities: [{ id: "light.1", name: "Ceiling", on: true }],
};

describe("RoomSheet", () => {
  it("renders room name and brightness", () => {
    render(<RoomSheet open room={fakeRoom} onOpenChange={() => {}} />);
    expect(screen.getByText("Living Room")).toBeInTheDocument();
    expect(screen.getByRole("slider")).toBeInTheDocument();
  });

  it("renders scene chips", () => {
    render(<RoomSheet open room={fakeRoom} onOpenChange={() => {}} />);
    expect(screen.getByText("Movie")).toBeInTheDocument();
    expect(screen.getByText("Dinner")).toBeInTheDocument();
  });
});
