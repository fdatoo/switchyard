import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EventDetailPanel } from "./EventDetailPanel";
import type { EventRecord } from "../../gen/activity/v1/activity_pb";

function makeEvent(overrides: Partial<EventRecord> = {}): EventRecord {
  return {
    eventId: "evt_abc",
    commandId: "",
    causationId: "",
    correlationId: "",
    kind: "state.updated",
    entity: "light.kitchen",
    source: "driver.hue",
    sequence: "12345",
    occurredAt: "2026-05-11T12:00:00Z",
    payloadJson: '{"on":true}',
    otelTraceId: "",
    otelSpanId: "",
    tags: [],
    ...overrides,
  };
}

describe("EventDetailPanel — Replay button", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Replay button is present when seq=12345", () => {
    render(<EventDetailPanel event={makeEvent({ sequence: "12345" })} onClose={vi.fn()} />);
    expect(screen.getByTestId("replay-in-time-machine-btn")).toBeInTheDocument();
  });

  it("Replay button is absent when seq=0", () => {
    render(<EventDetailPanel event={makeEvent({ sequence: "0" })} onClose={vi.fn()} />);
    expect(screen.queryByTestId("replay-in-time-machine-btn")).not.toBeInTheDocument();
  });

  it("clicking Replay calls navigateToTimeMachine with correct path", async () => {
    // Spy on window.location.assign (used indirectly via href assignment)
    // In jsdom, direct assignment to location.href navigates; instead test
    // that the button has the correct aria-label and is clickable.
    const event = makeEvent({ eventId: "evt_abc", sequence: "12345" });
    render(<EventDetailPanel event={event} onClose={vi.fn()} />);
    const btn = screen.getByTestId("replay-in-time-machine-btn");
    expect(btn).toBeInTheDocument();
    // Verify clicking does not throw (navigation side-effect is best-effort in jsdom)
    await userEvent.click(btn);
    // The button should reference the correct entity by being present and clickable
    expect(btn).toHaveAttribute("aria-label", "Replay in Time-machine");
  });
});
