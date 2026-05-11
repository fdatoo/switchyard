import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TopBar } from "./TopBar";
import type { ChainEvent } from "../../data/replay-client";

const STEPS: ChainEvent[] = [
  { eventId: "1", seq: "101", causationId: "", kind: "state.updated", entityId: "light.a", occurredAt: "" },
];

describe("TopBar", () => {
  it("renders title and subtitle", () => {
    render(
      <TopBar
        title="Time Machine"
        subtitle="light.kitchen"
        mode="event"
        steps={STEPS}
        onBack={vi.fn()}
        onExportTrace={vi.fn()}
      />,
    );
    expect(screen.getByText("Time Machine")).toBeInTheDocument();
    expect(screen.getByText("light.kitchen")).toBeInTheDocument();
  });

  it("calls onBack when back button clicked", async () => {
    const onBack = vi.fn();
    render(
      <TopBar
        title="Time Machine"
        subtitle=""
        mode="event"
        steps={STEPS}
        onBack={onBack}
        onExportTrace={vi.fn()}
      />,
    );
    await userEvent.click(screen.getByLabelText("Back"));
    expect(onBack).toHaveBeenCalledOnce();
  });

  it("export trace button is disabled when no steps", () => {
    render(
      <TopBar
        title="Time Machine"
        subtitle=""
        mode="event"
        steps={[]}
        onBack={vi.fn()}
        onExportTrace={vi.fn()}
      />,
    );
    expect(screen.getByLabelText("Export trace as NDJSON")).toBeDisabled();
  });
});
