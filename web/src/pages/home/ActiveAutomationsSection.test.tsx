import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ActiveAutomationsSection } from "./ActiveAutomationsSection";
import type { AutomationItem } from "./hooks/useHomeAutomations";

const mockAutomations: AutomationItem[] = [
  { id: "auto-1", name: "Lock front door at 11 PM", timeLabel: "in 47 min" },
  { id: "auto-2", name: "Morning routine", timeLabel: "6:30 AM" },
];

describe("ActiveAutomationsSection", () => {
  it('renders "Run now" buttons for each automation', () => {
    render(<ActiveAutomationsSection automations={mockAutomations} />);

    const buttons = screen.getAllByRole("button", { name: /run now/i });
    expect(buttons).toHaveLength(2);
  });

  it('"Run now" button does not throw — only console.warn side effect', () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => undefined);

    render(<ActiveAutomationsSection automations={mockAutomations} />);

    const buttons = screen.getAllByRole("button", { name: /run now/i });
    expect(() => fireEvent.click(buttons[0])).not.toThrow();

    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("TODO(plan-10)"),
      expect.anything(),
    );

    warnSpy.mockRestore();
  });

  it("renders automation names and time labels", () => {
    render(<ActiveAutomationsSection automations={mockAutomations} />);

    expect(screen.getByText("Lock front door at 11 PM")).toBeDefined();
    expect(screen.getByText("in 47 min")).toBeDefined();
    expect(screen.getByText("Morning routine")).toBeDefined();
  });

  it('"All automations" link points to /automations', () => {
    render(<ActiveAutomationsSection automations={mockAutomations} />);

    const link = screen.getByRole("link", { name: /all automations/i });
    expect(link.getAttribute("href")).toBe("/automations");
  });
});
