import { render, waitFor } from "@testing-library/react";
import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import { DisplayPage } from "./display.$id";

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

class MemoryLocalStorage {
  private store = new Map<string, string>();
  getItem(key: string): string | null { return this.store.get(key) ?? null; }
  setItem(key: string, value: string): void { this.store.set(key, value); }
  removeItem(key: string): void { this.store.delete(key); }
  clear(): void { this.store.clear(); }
  get length() { return this.store.size; }
  key(i: number): string | null { return [...this.store.keys()][i] ?? null; }
}

describe("DisplayPage", () => {
  let memLS: MemoryLocalStorage;
  let replaceSpy: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    memLS = new MemoryLocalStorage();
    vi.stubGlobal("localStorage", memLS);
    vi.stubGlobal("matchMedia", makeMatchMediaStub());
    replaceSpy = vi.fn();
    vi.stubGlobal("location", { ...window.location, replace: replaceSpy });
    vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise(() => {})));
    delete document.documentElement.dataset.theme;
    delete document.documentElement.dataset.language;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    delete document.documentElement.dataset.theme;
    delete document.documentElement.dataset.language;
  });

  it("redirects to /pair when no token in localStorage", async () => {
    render(<DisplayPage id="display-123" />);

    await waitFor(() => {
      expect(replaceSpy).toHaveBeenCalledWith("/pair?hint=display-123");
    });
  });

  it("renders ambient root with data-language=ambient when token is present", async () => {
    memLS.setItem("sy.display.display-123.token", "sydisp_token_abc");

    vi.stubGlobal("fetch", vi.fn().mockImplementation((url: string) => {
      if (url.includes("DisplayService/Get")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({
            display: {
              id: "display-123",
              deviceName: "Kitchen Wall",
              pageSlug: "home",
              alertThreshold: "ALERT_THRESHOLD_MEDIUM",
            },
          }),
        } as Response);
      }
      // SolarService and ActivityService → return non-resolving promise
      return new Promise(() => {});
    }));

    const { container } = render(<DisplayPage id="display-123" />);

    await waitFor(() => {
      const root = container.querySelector("[data-language='ambient']");
      expect(root).not.toBeNull();
    });
  });
});
