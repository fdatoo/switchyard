import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CausationRail } from "./CausationRail";
import type { ChainEvent } from "../../data/replay-client";

const STEPS: ChainEvent[] = [
  { eventId: "1", seq: "101", causationId: "", kind: "state.updated", entityId: "light.a", occurredAt: "2026-01-01T00:00:01.000Z" },
  { eventId: "2", seq: "102", causationId: "1", kind: "command.issued", entityId: "light.a", occurredAt: "2026-01-01T00:00:02.000Z" },
  { eventId: "3", seq: "103", causationId: "2", kind: "state.updated", entityId: "light.b", occurredAt: "2026-01-01T00:00:03.000Z" },
  { eventId: "4", seq: "104", causationId: "3", kind: "config.applied", entityId: "light.c", occurredAt: "2026-01-01T00:00:04.000Z" },
];

describe("CausationRail", () => {
  it("shows 'Causation chain' header in event mode", () => {
    render(
      <CausationRail steps={STEPS} currentIndex={0} mode="event" onSeek={vi.fn()} />,
    );
    expect(screen.getByTestId("rail-header")).toHaveTextContent("Causation chain");
  });

  it("shows 'Event window' header in window mode", () => {
    render(
      <CausationRail steps={STEPS} currentIndex={0} mode="window" onSeek={vi.fn()} />,
    );
    expect(screen.getByTestId("rail-header")).toHaveTextContent("Event window");
  });

  it("active class on correct row for currentIndex=1", () => {
    render(
      <CausationRail steps={STEPS} currentIndex={1} mode="event" onSeek={vi.fn()} />,
    );
    expect(screen.getByTestId("rail-row-1")).toHaveAttribute("aria-current", "step");
    expect(screen.getByTestId("rail-row-0")).not.toHaveAttribute("aria-current");
  });

  it("clicking third row fires onSeek(2)", async () => {
    const onSeek = vi.fn();
    render(
      <CausationRail steps={STEPS} currentIndex={0} mode="event" onSeek={onSeek} />,
    );
    await userEvent.click(screen.getByTestId("rail-row-2"));
    expect(onSeek).toHaveBeenCalledWith(2);
  });
});
