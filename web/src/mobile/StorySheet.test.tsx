import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StorySheet } from "./StorySheet";

const fakeStory = {
  id: "story-1",
  title: "Motion detected",
  whyInteresting: ["First motion in 4 hours", "Night-time rule matched"],
  events: [{ id: "e1", summary: "PIR sensor triggered" }],
  actions: ["Dismiss", "View in Activity"],
};

describe("StorySheet", () => {
  it("renders story title and why-interesting cards", () => {
    render(<StorySheet open story={fakeStory} onOpenChange={() => {}} />);
    expect(screen.getByText("Motion detected")).toBeInTheDocument();
    expect(screen.getByText("First motion in 4 hours")).toBeInTheDocument();
  });

  it("renders action buttons", () => {
    render(<StorySheet open story={fakeStory} onOpenChange={() => {}} />);
    expect(screen.getByRole("button", { name: "Dismiss" })).toBeInTheDocument();
  });
});
