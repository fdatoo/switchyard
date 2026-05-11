import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StoriesTab } from "./StoriesTab";
import type { Story } from "../../gen/activity/v1/activity_pb";

const MOCK_STORIES: Story[] = [
  {
    id: "story-1",
    title: "Kitchen lights turned on",
    innerEventIds: ["1", "2"],
    occurredAt: "2026-05-11T12:00:00Z",
    tags: [{ category: "failure", name: "command_failed", explanation: "The command failed." }],
    source: "cli:admin",
    entityIds: ["light/kitchen"],
  },
  {
    id: "story-2",
    title: "Bedroom automation triggered",
    innerEventIds: ["3", "4", "5"],
    occurredAt: "2026-05-11T11:00:00Z",
    tags: [{ category: "causation", name: "automation_triggered", explanation: "Automation ran." }],
    source: "automation:morning",
    entityIds: ["light/bedroom"],
  },
  {
    id: "story-3",
    title: "New device registered",
    innerEventIds: ["6"],
    occurredAt: "2026-05-11T10:00:00Z",
    tags: [{ category: "novelty", name: "first_seen_entity", explanation: "New device." }],
    source: "driver:zigbee",
    entityIds: ["sensor/new"],
  },
];

describe("StoriesTab", () => {
  it("renders three story cards when given three stories", () => {
    render(<StoriesTab stories={MOCK_STORIES} />);
    const cards = screen.getAllByRole("button");
    expect(cards).toHaveLength(3);
  });

  it("clicking card 2 populates the ContextRail with its title", async () => {
    const user = userEvent.setup();
    render(<StoriesTab stories={MOCK_STORIES} />);

    // Click the second card
    const cards = screen.getAllByRole("button");
    await user.click(cards[1]);

    // ContextRail should show the second story's title
    expect(screen.getByTestId("context-rail-title")).toHaveTextContent(
      "Bedroom automation triggered",
    );
  });

  it("shows 'No stories found' when stories is empty", () => {
    render(<StoriesTab stories={[]} />);
    expect(screen.getByText("No stories found.")).toBeInTheDocument();
  });
});
