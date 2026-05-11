import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StatusRowSection } from "./StatusRowSection";
import type { StatusPillItem } from "./hooks/useHomeStatus";

const mockGoodItems: StatusPillItem[] = [
  { id: "entities", label: "87 of 94 online", severity: "good" },
  { id: "automations", label: "12 automations active", severity: "neutral" },
];

const mockWarnItems: StatusPillItem[] = [
  { id: "entities", label: "87 of 94 online", severity: "good" },
  { id: "driver", label: "Z2M driver reconnecting", severity: "warn" },
];

describe("StatusRowSection", () => {
  it("renders a warn pill for reconnecting drivers", () => {
    render(<StatusRowSection items={mockWarnItems} />);

    const warnPill = screen.getByText("Z2M driver reconnecting");
    expect(warnPill).toBeDefined();
    expect(warnPill.getAttribute("data-severity")).toBe("warn");
  });

  it("omits driver pill when no driver alert", () => {
    render(<StatusRowSection items={mockGoodItems} />);

    expect(screen.queryByText(/reconnecting/i)).toBeNull();
    // Only good and neutral pills present
    const pills = screen.getAllByText(/.+/);
    const warnPills = pills.filter((el) => el.getAttribute("data-severity") === "warn");
    expect(warnPills).toHaveLength(0);
  });

  it("renders all provided pills", () => {
    render(<StatusRowSection items={mockWarnItems} />);

    expect(screen.getByText("87 of 94 online")).toBeDefined();
    expect(screen.getByText("Z2M driver reconnecting")).toBeDefined();
  });
});
