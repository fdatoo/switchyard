import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { RecentActivitySection } from "./RecentActivitySection";
import type { ActivityStory } from "./hooks/useHomeActivity";

const mockStories: ActivityStory[] = [
  {
    id: "a1",
    title: "Motion detected in Hallway",
    kindPill: "Alert",
    relativeTime: "2 min ago",
    severityColor: "var(--sy-color-warn)",
  },
  {
    id: "a2",
    title: "Front door locked automatically",
    kindPill: "Automation",
    relativeTime: "18 min ago",
    severityColor: "var(--sy-color-good)",
  },
  {
    id: "a3",
    title: "Office CO₂ above threshold",
    kindPill: "Alert",
    relativeTime: "1 hr ago",
    severityColor: "var(--sy-color-bad)",
  },
];

describe("RecentActivitySection", () => {
  it("renders exactly 3 activity rows", () => {
    render(<RecentActivitySection stories={mockStories} />);

    const rows = screen.getAllByTestId("activity-row");
    expect(rows).toHaveLength(3);
  });

  it('"All activity" link points to /activity', () => {
    render(<RecentActivitySection stories={mockStories} />);

    const link = screen.getByRole("link", { name: /all activity/i });
    expect(link.getAttribute("href")).toBe("/activity");
  });

  it("renders story title and relative time", () => {
    render(<RecentActivitySection stories={mockStories} />);

    expect(screen.getByText("Motion detected in Hallway")).toBeDefined();
    expect(screen.getByText("2 min ago")).toBeDefined();
  });
});
