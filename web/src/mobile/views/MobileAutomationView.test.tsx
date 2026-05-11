import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MobileAutomationView } from "./MobileAutomationView";

const fakeAutomation = {
  id: "a1",
  name: "Night mode",
  description: "Dims lights at 10pm",
  enabled: true,
  lastRun: "2026-05-10T22:00:00Z",
};

describe("MobileAutomationView", () => {
  it("renders automation name and read-only banner", () => {
    render(<MobileAutomationView automation={fakeAutomation} />);
    expect(screen.getByText("Night mode")).toBeInTheDocument();
    expect(screen.getByText(/editing on a larger screen/i)).toBeInTheDocument();
  });

  it("shows Run, Enable/Disable, and View buttons", () => {
    render(<MobileAutomationView automation={fakeAutomation} />);
    expect(screen.getByRole("button", { name: /run/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /disable/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /view/i })).toBeInTheDocument();
  });
});
