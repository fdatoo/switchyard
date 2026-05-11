import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { markDaemonUnreachable, resetDaemonConnectionForTest } from "@/data/daemon-connection";
import { ReconnectingBanner } from "./ReconnectingBanner";

describe("ReconnectingBanner", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    resetDaemonConnectionForTest();
  });

  it("shows the persistent retry state while the daemon is unreachable", () => {
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>(() => undefined)));
    markDaemonUnreachable(new Error("connection refused"));

    const { unmount } = render(<ReconnectingBanner retryIntervalMs={30_000} />);

    expect(screen.getByRole("status")).toHaveTextContent("Reconnecting to gohome");
    expect(screen.getByText("Retrying health check...")).toBeVisible();
    unmount();
  });
});
