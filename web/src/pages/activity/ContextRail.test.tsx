import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ContextRail } from "./ContextRail";
import type { Story } from "../../gen/activity/v1/activity_pb";

const STORY_WITH_EVENTS: Story = {
  id: "story-cx",
  title: "Command issued in kitchen",
  innerEventIds: ["10", "11"],
  occurredAt: "2026-05-11T12:00:00Z",
  tags: [
    {
      category: "failure",
      name: "command_failed",
      explanation: "The command failed unexpectedly.",
    },
  ],
  source: "cli:admin",
  entityIds: ["light/kitchen"],
};

describe("ContextRail", () => {
  it("shows placeholder when no story is selected", () => {
    render(<ContextRail story={null} />);
    expect(screen.getByText("Select a story to see details")).toBeInTheDocument();
  });

  it("renders story title when a story is provided", () => {
    render(<ContextRail story={STORY_WITH_EVENTS} />);
    expect(screen.getByTestId("context-rail-title")).toHaveTextContent(
      "Command issued in kitchen",
    );
  });

  it("renders one why-item card for the single tag", () => {
    render(<ContextRail story={STORY_WITH_EVENTS} />);
    const whyItems = screen.getAllByTestId("why-item");
    expect(whyItems).toHaveLength(1);
    expect(whyItems[0]).toHaveTextContent("The command failed unexpectedly.");
  });

  it("renders two timeline rows for two inner events", () => {
    render(<ContextRail story={STORY_WITH_EVENTS} />);
    const rows = screen.getAllByTestId("timeline-row");
    expect(rows).toHaveLength(2);
  });
});
