import { describe, it, expect } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AllEventsTab } from "./AllEventsTab";
import type { EventRecord } from "../../gen/activity/v1/activity_pb";

const MOCK_EVENTS: EventRecord[] = [
  {
    eventId: "ev-1",
    kind: "cmd.issued",
    entity: "light/kitchen",
    source: "cli:admin",
    occurredAt: "2026-05-11T12:00:00Z",
    sequence: "1",
    correlationId: "c1",
    causationId: "",
    commandId: "",
    otelTraceId: "",
    otelSpanId: "",
    payloadJson: '{"key":"value"}',
    tags: [],
  },
  {
    eventId: "ev-2",
    kind: "state_changed",
    entity: "light/bedroom",
    source: "driver:zigbee",
    occurredAt: "2026-05-11T11:55:00Z",
    sequence: "2",
    correlationId: "c2",
    causationId: "",
    commandId: "",
    otelTraceId: "",
    otelSpanId: "",
    payloadJson: "",
    tags: [],
  },
  {
    eventId: "ev-3",
    kind: "cmd.failed",
    entity: "switch/garage",
    source: "automation:ev",
    occurredAt: "2026-05-11T11:50:00Z",
    sequence: "3",
    correlationId: "c3",
    causationId: "",
    commandId: "",
    otelTraceId: "",
    otelSpanId: "",
    payloadJson: "",
    tags: [{ category: "failure", name: "command_failed", explanation: "Failed." }],
  },
];

describe("AllEventsTab", () => {
  it("renders FacetRail and EventTable with mock data", () => {
    render(<AllEventsTab events={MOCK_EVENTS} />);

    expect(screen.getByTestId("facet-rail")).toBeInTheDocument();
    expect(screen.getByTestId("event-table")).toBeInTheDocument();
    expect(screen.getAllByTestId("event-row")).toHaveLength(3);
  });

  it("renders Sparkline SVG", () => {
    render(<AllEventsTab events={MOCK_EVENTS} />);
    expect(screen.getByTestId("sparkline-svg")).toBeInTheDocument();
  });

  it("clicking a facet value filters the event table", async () => {
    const user = userEvent.setup();
    render(<AllEventsTab events={MOCK_EVENTS} />);

    // Click the "cmd.issued" kind facet
    const facetRail = screen.getByTestId("facet-rail");
    const cmdBtn = within(facetRail).getByText("cmd.issued");
    await user.click(cmdBtn.closest("button")!);

    // Only the cmd.issued event should remain
    expect(screen.getAllByTestId("event-row")).toHaveLength(1);
  });

  it("clicking a table row opens the event detail panel with correct event_id", async () => {
    const user = userEvent.setup();
    render(<AllEventsTab events={MOCK_EVENTS} />);

    const rows = screen.getAllByTestId("event-row");
    await user.click(rows[0]);

    expect(screen.getByTestId("event-detail-panel")).toBeInTheDocument();
    expect(screen.getByTestId("event-id")).toHaveTextContent("ev-1");
  });

  it("shows 'No events found' when all events are filtered out", async () => {
    const user = userEvent.setup();
    render(<AllEventsTab events={MOCK_EVENTS} />);

    const input = screen.getByTestId("query-input");
    await user.type(input, "zzznomatch");

    expect(screen.getByText("No events found.")).toBeInTheDocument();
  });
});
