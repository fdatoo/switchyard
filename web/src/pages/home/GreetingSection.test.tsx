import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { GreetingSection } from "./GreetingSection";

afterEach(() => {
  vi.useRealTimers();
});

describe("GreetingSection", () => {
  it("renders the greeting and calm status together", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 0, 15, 14, 0, 0)); // 14:00

    render(<GreetingSection alertCount={0} />);

    const heading = screen.getByRole("heading", { level: 1 });
    expect(heading.textContent).toContain("Good afternoon");
    expect(heading.textContent).toContain("everything looks calm");
  });

  it("applies no edit affordances — no buttons rendered", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 0, 15, 14, 0, 0));

    render(<GreetingSection alertCount={0} />);

    expect(screen.queryByRole("button")).toBeNull();
  });

  it("renders alert copy when alertCount is 2", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 0, 15, 9, 0, 0));

    render(<GreetingSection alertCount={2} />);

    const heading = screen.getByRole("heading", { level: 1 });
    expect(heading.textContent).toContain("2 things need attention");
  });
});
