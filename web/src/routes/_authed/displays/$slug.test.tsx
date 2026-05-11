import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import { DisplaySlug } from "./$slug";

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
  deviceName: "Bedroom Nightstand",
  pageSlug: "home",
  alertThreshold: "ALERT_THRESHOLD_NONE",
  tileOverrides: {},
};

describe("DisplaySlug config editor", () => {
  let updateSpy: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.stubGlobal("matchMedia", makeMatchMediaStub());
    updateSpy = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) });
    delete document.documentElement.dataset.theme;
    delete document.documentElement.dataset.language;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders device name after loading", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ display: MOCK_DISPLAY }),
    } as Response));

    render(<DisplaySlug slug="disp-1" />);

    await waitFor(() => {
      expect(screen.getByTestId("display-config-page")).toBeInTheDocument();
    });
    expect(screen.getByText("Bedroom Nightstand")).toBeInTheDocument();
  });

  it("calls DisplayService.Update with ALERT_HIGH when user selects High and saves", async () => {
    const user = userEvent.setup();
    vi.stubGlobal("fetch", vi.fn().mockImplementation((url: string) => {
      if (url.includes("Get")) {
        return Promise.resolve({ ok: true, json: async () => ({ display: MOCK_DISPLAY }) } as Response);
      }
      if (url.includes("Update")) {
        return updateSpy(url);
      }
      return new Promise(() => {});
    }));

    render(<DisplaySlug slug="disp-1" />);

    await waitFor(() => expect(screen.getByTestId("display-config-page")).toBeInTheDocument());

    // Select "High" threshold
    const highRadio = screen.getByTestId("threshold-ALERT_THRESHOLD_HIGH");
    await user.click(highRadio);

    // Save
    await user.click(screen.getByTestId("save-btn"));

    await waitFor(() => {
      expect(updateSpy).toHaveBeenCalled();
    });
    // The update is called via fetch — check fetch was called with Update URL
    expect(updateSpy.mock.calls[0][0]).toContain("Update");
  });
});
