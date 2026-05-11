import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { MobileActivity } from "./MobileActivity";

const fakeStories = [
  { id: "s1", title: "Motion detected", whyInteresting: [], events: [], actions: [] },
  { id: "s2", title: "Lights off at midnight", whyInteresting: [], events: [], actions: [] },
];

vi.mock("@/hooks/useActivityFeed", () => ({
  useActivityFeed: () => ({ stories: fakeStories, isLoading: false }),
}));

describe("MobileActivity", () => {
  it("renders story cards", () => {
    render(<MobileActivity />);
    expect(screen.getByText("Motion detected")).toBeInTheDocument();
  });

  it("tapping a story opens StorySheet", async () => {
    const user = userEvent.setup();
    render(<MobileActivity />);
    await user.click(screen.getByText("Motion detected"));
    // StorySheet title appears in portal
    expect(screen.getAllByText("Motion detected").length).toBeGreaterThan(1);
  });
});
