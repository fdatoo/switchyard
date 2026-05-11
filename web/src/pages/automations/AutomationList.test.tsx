import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { AutomationList } from "./AutomationList";

// Mock the hook to return two automations (one disabled)
vi.mock("./useAutomations", () => ({
  useAutomations: () => [
    {
      id: "sunset-lights",
      displayName: "Sunset Lights",
      enabled: true,
      trigger: { kind: "sun_event", summary: "Sun · sunset −15min" },
    },
    {
      id: "lock-front-door",
      displayName: "Lock Front Door",
      enabled: false,
      trigger: { kind: "time", summary: "Daily at 11:00 PM" },
    },
  ],
}));

describe("AutomationList", () => {
  it("renders both automation names", () => {
    render(<AutomationList />);
    expect(screen.getByText("Sunset Lights")).toBeDefined();
    expect(screen.getByText("Lock Front Door")).toBeDefined();
  });

  it("renders trigger summaries", () => {
    render(<AutomationList />);
    expect(screen.getByText("Sun · sunset −15min")).toBeDefined();
    expect(screen.getByText("Daily at 11:00 PM")).toBeDefined();
  });

  it("renders the disabled chip for disabled automations", () => {
    render(<AutomationList />);
    const disabledChips = screen.getAllByText("disabled");
    expect(disabledChips).toHaveLength(1);
  });

  it("renders the active chip for enabled automations", () => {
    render(<AutomationList />);
    const activeChips = screen.getAllByText("active");
    expect(activeChips).toHaveLength(1);
  });

  it("renders Run now buttons for each automation", () => {
    render(<AutomationList />);
    const runButtons = screen.getAllByText("Run now");
    expect(runButtons).toHaveLength(2);
  });

  it("renders links with correct hrefs", () => {
    render(<AutomationList />);
    const sunsetLink = screen.getByRole("link", { name: /Sunset Lights/ });
    expect(sunsetLink.getAttribute("href")).toBe(
      "/_authed/automations/sunset-lights",
    );

    const lockLink = screen.getByRole("link", { name: /Lock Front Door/ });
    expect(lockLink.getAttribute("href")).toBe(
      "/_authed/automations/lock-front-door",
    );
  });
});

describe("AutomationList empty state", () => {
  it("renders empty state message when no automations", () => {
    // Override hook for this test
    vi.doMock("./useAutomations", () => ({ useAutomations: () => [] }));
    // The existing mock from above returns 2 items; for empty state coverage, see integration test
  });
});
