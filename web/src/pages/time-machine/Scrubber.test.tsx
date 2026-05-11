import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Scrubber } from "./Scrubber";
import type { PlaySpeed } from "./Scrubber";
import type { ChainEvent } from "../../data/replay-client";

const STEPS: ChainEvent[] = [
  { eventId: "1", seq: "101", causationId: "", kind: "state.updated", entityId: "light.a", occurredAt: "2026-01-01T00:00:01.000Z" },
  { eventId: "2", seq: "102", causationId: "1", kind: "command.issued", entityId: "light.a", occurredAt: "2026-01-01T00:00:02.000Z" },
  { eventId: "3", seq: "103", causationId: "2", kind: "state.updated", entityId: "light.b", occurredAt: "2026-01-01T00:00:03.000Z" },
  { eventId: "4", seq: "104", causationId: "3", kind: "config.applied", entityId: "light.c", occurredAt: "2026-01-01T00:00:04.000Z" },
  { eventId: "5", seq: "105", causationId: "4", kind: "state.updated", entityId: "light.d", occurredAt: "2026-01-01T00:00:05.000Z" },
];

function renderScrubber(props: Partial<Parameters<typeof Scrubber>[0]> = {}) {
  const defaults = {
    steps: STEPS,
    currentIndex: 0,
    playing: false,
    speed: 1 as PlaySpeed,
    onPlay: vi.fn(),
    onPause: vi.fn(),
    onNext: vi.fn(),
    onPrev: vi.fn(),
    onFirst: vi.fn(),
    onLast: vi.fn(),
    onSeek: vi.fn(),
    onSpeedChange: vi.fn(),
  };
  return render(<Scrubber {...defaults} {...props} />);
}

describe("Scrubber", () => {
  it("shows 'step 3 of 5' when currentIndex=2 on 5 steps", () => {
    renderScrubber({ currentIndex: 2 });
    expect(screen.getByTestId("pos-label")).toHaveTextContent("step 3 of 5");
  });

  it("clicking › fires onNext", async () => {
    const onNext = vi.fn();
    renderScrubber({ onNext, currentIndex: 1 });
    await userEvent.click(screen.getByTestId("next-btn"));
    expect(onNext).toHaveBeenCalledOnce();
  });

  it("clicking a speed segment fires onSpeedChange with that multiplier", async () => {
    const onSpeedChange = vi.fn();
    renderScrubber({ onSpeedChange });
    // Click the 4× speed button
    await userEvent.click(screen.getByLabelText("4× speed"));
    expect(onSpeedChange).toHaveBeenCalledWith(4);
  });

  it("clicking 0.25× speed fires onSpeedChange(0.25)", async () => {
    const onSpeedChange = vi.fn();
    renderScrubber({ onSpeedChange });
    await userEvent.click(screen.getByLabelText("0.25× speed"));
    expect(onSpeedChange).toHaveBeenCalledWith(0.25);
  });

  it("renders 5 dots for 5 steps", () => {
    renderScrubber();
    expect(screen.getAllByTestId(/^dot-/)).toHaveLength(5);
  });
});
