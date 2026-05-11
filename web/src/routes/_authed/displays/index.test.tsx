import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import { DisplaysIndex } from "./index";

function makeMatchMediaStub() {
  return (query: string): MediaQueryList => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  });
}

const MOCK_DISPLAY = {
  id: "disp-1",
  deviceName: "Kitchen Wall",
  pageSlug: "home",
  alertThreshold: "ALERT_THRESHOLD_MEDIUM",
  tileOverrides: {},
};

describe("DisplaysIndex", () => {
  beforeEach(() => {
    vi.stubGlobal("matchMedia", makeMatchMediaStub());
    delete document.documentElement.dataset.theme;
    delete document.documentElement.dataset.language;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders device name and page slug for a mocked display", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ displays: [MOCK_DISPLAY] }),
    } as Response));

    render(<DisplaysIndex />);

    await waitFor(() => {
      expect(screen.getByTestId(`display-name-${MOCK_DISPLAY.id}`)).toHaveTextContent("Kitchen Wall");
    });
    expect(screen.getByTestId("displays-table")).toBeInTheDocument();
  });

  it("shows 'Pair new display' button", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ displays: [] }),
    } as Response));

    render(<DisplaysIndex />);
    await waitFor(() => expect(screen.queryByText("Loading…")).toBeNull());
    expect(screen.getByTestId("pair-new-display-btn")).toBeInTheDocument();
  });

  it("opens pairing modal on 'Pair new display' click", async () => {
    const user = userEvent.setup();
    vi.stubGlobal("fetch", vi.fn().mockImplementation((url: string) => {
      if (url.includes("List")) {
        return Promise.resolve({ ok: true, json: async () => ({ displays: [] }) } as Response);
      }
      if (url.includes("Pair")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({ code: "123456", expires_at: Math.floor(Date.now() / 1000) + 300 }),
        } as Response);
      }
      return new Promise(() => {});
    }));

    render(<DisplaysIndex />);
    await waitFor(() => expect(screen.queryByText("Loading…")).toBeNull());

    await user.click(screen.getByTestId("pair-new-display-btn"));

    await waitFor(() => {
      expect(screen.getByTestId("pairing-modal")).toBeInTheDocument();
    });
    // Code should be displayed
    await waitFor(() => {
      expect(screen.getByTestId("pair-code-display")).toHaveTextContent("123456");
    });
  });
});
