import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { MobileHome } from "./MobileHome";

vi.mock("@/hooks/useHomeSummary", () => ({
  useHomeSummary: () => ({
    stats: [
      { id: "lights-on", label: "Lights on", value: "4" },
      { id: "temp", label: "Temperature", value: "21°C" },
      { id: "open-doors", label: "Open doors", value: "1" },
      { id: "automations", label: "Automations", value: "12" },
    ],
    rooms: [
      { slug: "living", name: "Living Room" },
      { slug: "bedroom", name: "Bedroom" },
    ],
    recentStories: [],
  }),
}));

describe("MobileHome", () => {
  it("renders 4 stat tiles", () => {
    render(<MobileHome />);
    expect(screen.getByText("Lights on")).toBeInTheDocument();
    expect(screen.getByText("Temperature")).toBeInTheDocument();
    expect(screen.getByText("Open doors")).toBeInTheDocument();
    expect(screen.getByText("Automations")).toBeInTheDocument();
  });

  it("renders rooms grid", () => {
    render(<MobileHome />);
    expect(screen.getByText("Living Room")).toBeInTheDocument();
    expect(screen.getByText("Bedroom")).toBeInTheDocument();
  });
});
