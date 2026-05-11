import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { StatePane } from "./StatePane";
import type { EntityState, StateDiff } from "../../data/replay-client";

const ENTITIES: EntityState[] = [
  { entityId: "light.kitchen", fields: { brightness: "64", on: "true" } },
  { entityId: "climate.living", fields: { temperature: "22", mode: "heat" } },
  { entityId: "switch.fan", fields: { on: "false" } },
];

const DIFF: StateDiff = {
  entityDiffs: [
    {
      entityId: "light.kitchen",
      fieldDiffs: [{ field: "brightness", was: "18", now: "64" }],
    },
  ],
};

const EMPTY_DIFF: StateDiff = { entityDiffs: [] };

describe("StatePane — All mode", () => {
  it("renders all 3 entities and marks changed one with ring", () => {
    render(
      <StatePane
        entities={ENTITIES}
        diff={DIFF}
        whyInteresting=""
        mode="all"
        onModeChange={vi.fn()}
      />,
    );
    expect(screen.getByTestId("entity-card-light.kitchen")).toBeInTheDocument();
    expect(screen.getByTestId("entity-card-climate.living")).toBeInTheDocument();
    expect(screen.getByTestId("entity-card-switch.fan")).toBeInTheDocument();
    expect(screen.getByTestId("changed-label-light.kitchen")).toBeInTheDocument();
  });
});

describe("StatePane — Affected only mode", () => {
  it("renders only the 1 changed entity", () => {
    render(
      <StatePane
        entities={ENTITIES}
        diff={DIFF}
        whyInteresting=""
        mode="affected"
        onModeChange={vi.fn()}
      />,
    );
    expect(screen.getByTestId("entity-card-light.kitchen")).toBeInTheDocument();
    expect(screen.queryByTestId("entity-card-climate.living")).not.toBeInTheDocument();
    expect(screen.queryByTestId("entity-card-switch.fan")).not.toBeInTheDocument();
  });
});

describe("StatePane — Diff mode", () => {
  it("shows was and now for changed field", () => {
    render(
      <StatePane
        entities={ENTITIES}
        diff={DIFF}
        whyInteresting=""
        mode="diff"
        onModeChange={vi.fn()}
      />,
    );
    // The changed entity should show brightness diff.
    expect(screen.getByText("18")).toBeInTheDocument(); // was
    expect(screen.getByText("64")).toBeInTheDocument(); // now
  });
});

describe("StatePane — why interesting panel", () => {
  it("renders panel when whyInteresting is non-empty", () => {
    render(
      <StatePane
        entities={ENTITIES}
        diff={EMPTY_DIFF}
        whyInteresting="slow driver ack"
        mode="all"
        onModeChange={vi.fn()}
      />,
    );
    expect(screen.getByTestId("why-interesting-panel")).toBeInTheDocument();
    expect(screen.getByText("slow driver ack")).toBeInTheDocument();
    expect(screen.getByText("Why is this step interesting?")).toBeInTheDocument();
  });

  it("does not render panel when whyInteresting is empty", () => {
    render(
      <StatePane
        entities={ENTITIES}
        diff={EMPTY_DIFF}
        whyInteresting=""
        mode="all"
        onModeChange={vi.fn()}
      />,
    );
    expect(screen.queryByTestId("why-interesting-panel")).not.toBeInTheDocument();
  });
});
