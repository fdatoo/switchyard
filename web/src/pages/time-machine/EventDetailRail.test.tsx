import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { EventDetailRail } from "./EventDetailRail";
import type { ChainEvent, LoadAtSeqResponse } from "../../data/replay-client";

const STEP: ChainEvent = {
  eventId: "evt_abc",
  seq: "102",
  causationId: "evt_101",
  kind: "state.updated",
  entityId: "light.kitchen",
  occurredAt: "2026-01-01T12:00:00.000Z",
};

const STATE: LoadAtSeqResponse = {
  seq: "102",
  entities: [{ entityId: "light.kitchen", fields: { brightness: "64" } }],
  diff: {
    entityDiffs: [
      {
        entityId: "light.kitchen",
        fieldDiffs: [{ field: "brightness", was: "18", now: "64" }],
      },
    ],
  },
  eventId: "evt_abc",
  kind: "state.updated",
  entityId: "light.kitchen",
  source: "driver.hue",
  causationId: "evt_101",
  correlationId: "corr_xyz",
  emitter: "driver.hue",
  spanId: "span_001",
  occurredAt: "2026-01-01T12:00:00Z",
  payloadJson: '{"stateChanged":{"attributes":{"light":{"on":true,"brightness":64}}}}',
  whyInteresting: "",
};

describe("EventDetailRail", () => {
  it("renders all five section headers", () => {
    render(<EventDetailRail step={STEP} state={STATE} />);
    expect(screen.getByTestId("diff-heading")).toBeInTheDocument();
    expect(screen.getByTestId("identity-heading")).toBeInTheDocument();
    expect(screen.getByTestId("source-heading")).toBeInTheDocument();
    expect(screen.getByTestId("payload-heading")).toBeInTheDocument();
    // Kind chip + entity are in the first section (no explicit heading)
    expect(screen.getByTestId("kind-chip")).toHaveTextContent("state.updated");
  });

  it("renders event_id value in Identity section", () => {
    render(<EventDetailRail step={STEP} state={STATE} />);
    expect(screen.getByTestId("event-id-value")).toHaveTextContent("evt_abc");
  });

  it("renders payload block with formatted JSON", () => {
    render(<EventDetailRail step={STEP} state={STATE} />);
    const payloadBlock = screen.getByTestId("payload-block");
    expect(payloadBlock).toBeInTheDocument();
    // The block should contain the JSON content
    expect(payloadBlock.textContent).toContain("stateChanged");
  });
});
