import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { HomePage } from "./HomePage";

afterEach(() => {
  vi.useRealTimers();
});

describe("HomePage", () => {
  it("renders all six sections", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 0, 15, 14, 0, 0));

    render(<HomePage />);

    // 1. Greeting section — <h1> with greeting text
    const heading = screen.getByRole("heading", { level: 1 });
    expect(heading.textContent).toContain("Good afternoon");

    // 3. Right Now strip — at least one stat tile
    const statTiles = screen.getAllByTestId("stat-tile");
    expect(statTiles.length).toBeGreaterThan(0);

    // 4. Rooms grid — at least one room tile
    const roomTiles = screen.getAllByTestId("room-tile");
    expect(roomTiles.length).toBeGreaterThan(0);

    // 5. Recent activity — at least one activity row
    const activityRows = screen.getAllByTestId("activity-row");
    expect(activityRows.length).toBeGreaterThan(0);

    // 6. Active automations — "Run now" button present
    const runNowButtons = screen.getAllByRole("button", { name: /run now/i });
    expect(runNowButtons.length).toBeGreaterThan(0);
  });

  it("has no edit button", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 0, 15, 14, 0, 0));

    render(<HomePage />);

    expect(screen.queryByRole("button", { name: /edit/i })).toBeNull();
  });
});
