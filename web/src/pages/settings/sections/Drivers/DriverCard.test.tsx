import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { DriverCard } from "./DriverCard";
import type { DriverSummary } from "@/data/driver-management-client";
import { SCOPE_TOOLTIP } from "./ExpandedDetail";

// Mock the driver management client
vi.mock("@/data/driver-management-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/data/driver-management-client")>();
  return {
    ...actual,
    driverClient: {
      logs: vi.fn().mockResolvedValue([
        "[2026-05-11T10:00:00Z] info: starting z2m",
        "[2026-05-11T10:00:01Z] info: ready",
      ]),
      stop: vi.fn().mockResolvedValue(undefined),
      restart: vi.fn().mockResolvedValue(undefined),
      list: vi.fn().mockResolvedValue({ running: [], available: [] }),
      get: vi.fn().mockResolvedValue(null),
    },
  };
});

const mockDriver: DriverSummary = {
  id: "z2m",
  pack: "@switchyard/z2m",
  version: "2.0.1",
  status: "reconnecting",
  uptimeSeconds: 7200,
  pid: 12345,
  socket: "/run/switchyard/z2m.sock",
  configFile: "/etc/switchyard/z2m.pkl",
  otelSpan: "trace-abc123",
  entityCount: 17,
  eventsPerDay: 1234,
  lastCmdAckMs: 42,
  reconnectsToday: 2,
  reconnectingSince: "",
};

describe("DriverCard", () => {
  const onToggle = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders pack name and version", () => {
    render(
      <DriverCard
        driver={mockDriver}
        expanded={false}
        onToggle={onToggle}
        hasWriteScope={true}
      />,
    );
    expect(screen.getByText("@switchyard/z2m")).toBeInTheDocument();
    expect(screen.getByText(/v2\.0\.1/)).toBeInTheDocument();
  });

  it("renders the status chip", () => {
    render(
      <DriverCard
        driver={mockDriver}
        expanded={false}
        onToggle={onToggle}
        hasWriteScope={true}
      />,
    );
    expect(screen.getByText("reconnecting")).toBeInTheDocument();
  });

  it("calls onToggle when clicked", async () => {
    render(
      <DriverCard
        driver={mockDriver}
        expanded={false}
        onToggle={onToggle}
        hasWriteScope={true}
      />,
    );
    // The row button has aria-expanded=false; click it to trigger expand
    const rowBtn = screen.getByRole("button", { name: /@switchyard\/z2m/ });
    fireEvent.click(rowBtn);
    // onToggle is called after the async logs fetch resolves
    await waitFor(() => {
      expect(onToggle).toHaveBeenCalledWith("z2m");
    });
  });

  it("shows Stop driver and Restart disabled with scope tooltip when hasWriteScope=false", async () => {
    render(
      <DriverCard
        driver={mockDriver}
        expanded={true}
        onToggle={onToggle}
        hasWriteScope={false}
      />,
    );
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Stop driver" })).toBeInTheDocument();
    });
    const stopBtn = screen.getByRole("button", { name: "Stop driver" });
    const restartBtn = screen.getByRole("button", { name: "Restart" });
    expect(stopBtn).toBeDisabled();
    expect(restartBtn).toBeDisabled();
    expect(stopBtn).toHaveAttribute("title", SCOPE_TOOLTIP);
    expect(restartBtn).toHaveAttribute("title", SCOPE_TOOLTIP);
  });

  it("shows Stop driver and Restart enabled when hasWriteScope=true", async () => {
    render(
      <DriverCard
        driver={mockDriver}
        expanded={true}
        onToggle={onToggle}
        hasWriteScope={true}
      />,
    );
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Stop driver" })).toBeInTheDocument();
    });
    const stopBtn = screen.getByRole("button", { name: "Stop driver" });
    const restartBtn = screen.getByRole("button", { name: "Restart" });
    expect(stopBtn).not.toBeDisabled();
    expect(restartBtn).not.toBeDisabled();
  });
});
