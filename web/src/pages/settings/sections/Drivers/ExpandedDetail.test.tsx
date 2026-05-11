import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { ExpandedDetail, SCOPE_TOOLTIP } from "./ExpandedDetail";
import type { DriverSummary } from "@/data/driver-management-client";

const mockDriver: DriverSummary = {
  id: "z2m",
  pack: "@switchyard/z2m",
  version: "2.0.1",
  status: "reconnecting",
  uptimeSeconds: 3600,
  pid: 12345,
  socket: "/run/switchyard/z2m.sock",
  configFile: "/etc/switchyard/z2m.pkl",
  otelSpan: "trace-abc123",
  entityCount: 17,
  eventsPerDay: 1234.5,
  lastCmdAckMs: 42,
  reconnectsToday: 2,
  reconnectingSince: "",
};

const mockLogLines = [
  "[2026-05-11T10:00:00Z] info: starting zigbee2mqtt",
  "[2026-05-11T10:00:01Z] info: coordinator detected",
  "[2026-05-11T10:00:07Z] info: reconnected",
];

describe("ExpandedDetail", () => {
  it("renders identity rows for pack, version, pid, socket, config, otel span", () => {
    render(
      <ExpandedDetail
        driver={mockDriver}
        logLines={mockLogLines}
        hasWriteScope={true}
        onStop={vi.fn()}
        onRestart={vi.fn()}
      />,
    );
    expect(screen.getByText("@switchyard/z2m")).toBeInTheDocument();
    expect(screen.getByText("2.0.1")).toBeInTheDocument();
    expect(screen.getByText("12345")).toBeInTheDocument();
    expect(screen.getByText("/run/switchyard/z2m.sock")).toBeInTheDocument();
    expect(screen.getByText("/etc/switchyard/z2m.pkl")).toBeInTheDocument();
    expect(screen.getByText("trace-abc123")).toBeInTheDocument();
  });

  it("renders log lines in the pre block", () => {
    render(
      <ExpandedDetail
        driver={mockDriver}
        logLines={mockLogLines}
        hasWriteScope={true}
        onStop={vi.fn()}
        onRestart={vi.fn()}
      />,
    );
    expect(screen.getByText(/starting zigbee2mqtt/)).toBeInTheDocument();
    expect(screen.getByText(/coordinator detected/)).toBeInTheDocument();
  });

  it("action links point to correct hrefs", () => {
    render(
      <ExpandedDetail
        driver={mockDriver}
        logLines={mockLogLines}
        hasWriteScope={true}
        onStop={vi.fn()}
        onRestart={vi.fn()}
      />,
    );
    const timeMachineLink = screen.getByRole("link", { name: "Open in Time-machine" });
    expect(timeMachineLink).toHaveAttribute("href", "/activity?driver=z2m");

    const inspectLink = screen.getByRole("link", { name: "Inspect entities" });
    expect(inspectLink).toHaveAttribute("href", "/devices?driver=z2m");
  });

  it("Stop driver and Restart are disabled with scope tooltip when hasWriteScope=false", () => {
    render(
      <ExpandedDetail
        driver={mockDriver}
        logLines={mockLogLines}
        hasWriteScope={false}
        onStop={vi.fn()}
        onRestart={vi.fn()}
      />,
    );
    const stopBtn = screen.getByRole("button", { name: "Stop driver" });
    const restartBtn = screen.getByRole("button", { name: "Restart" });
    expect(stopBtn).toBeDisabled();
    expect(restartBtn).toBeDisabled();
    expect(stopBtn).toHaveAttribute("title", SCOPE_TOOLTIP);
    expect(restartBtn).toHaveAttribute("title", SCOPE_TOOLTIP);
  });

  it("Stop driver and Restart are enabled when hasWriteScope=true", () => {
    render(
      <ExpandedDetail
        driver={mockDriver}
        logLines={mockLogLines}
        hasWriteScope={true}
        onStop={vi.fn()}
        onRestart={vi.fn()}
      />,
    );
    const stopBtn = screen.getByRole("button", { name: "Stop driver" });
    const restartBtn = screen.getByRole("button", { name: "Restart" });
    expect(stopBtn).not.toBeDisabled();
    expect(restartBtn).not.toBeDisabled();
  });
});
