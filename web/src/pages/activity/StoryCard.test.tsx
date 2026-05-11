import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { StoryCard } from "./StoryCard";
import type { Story } from "../../gen/activity/v1/activity_pb";

const STORY_WITH_TAGS: Story = {
  id: "test-story",
  title: "Test story",
  innerEventIds: ["1"],
  occurredAt: "2026-05-11T12:00:00Z",
  tags: [
    { category: "failure", name: "command_failed", explanation: "Failed." },
    { category: "causation", name: "high_fan_out", explanation: "Fan-out." },
  ],
  source: "cli:admin",
  entityIds: ["light/kitchen"],
};

describe("StoryCard", () => {
  it("renders failure and causation badges with correct data-interesting-category", () => {
    render(<StoryCard story={STORY_WITH_TAGS} onSelect={vi.fn()} />);

    const failureBadge = screen.getByText("command_failed");
    expect(failureBadge).toHaveAttribute("data-interesting-category", "failure");

    const causationBadge = screen.getByText("high_fan_out");
    expect(causationBadge).toHaveAttribute("data-interesting-category", "causation");
  });

  it("renders no style attribute on the badge elements (no raw colors)", () => {
    render(<StoryCard story={STORY_WITH_TAGS} onSelect={vi.fn()} />);

    const badges = screen.getAllByRole("status");
    for (const badge of badges) {
      expect(badge).not.toHaveAttribute("style");
    }
  });

  it("calls onSelect when clicked", async () => {
    const onSelect = vi.fn();
    render(<StoryCard story={STORY_WITH_TAGS} onSelect={onSelect} />);

    const card = screen.getByRole("button");
    card.click();

    expect(onSelect).toHaveBeenCalledWith(STORY_WITH_TAGS);
  });
});
