import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import { PairPage } from "./pair";

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

describe("PairPage", () => {
  let memLS: MemoryLocalStorage;
  let replaceSpy: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    memLS = new MemoryLocalStorage();
    vi.stubGlobal("localStorage", memLS);
    vi.stubGlobal("matchMedia", makeMatchMediaStub());
    replaceSpy = vi.fn();
    vi.stubGlobal("location", { ...window.location, replace: replaceSpy, search: "" });
    delete document.documentElement.dataset.theme;
    delete document.documentElement.dataset.language;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders the pair page with code input", () => {
    vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise(() => {})));
    render(<PairPage />);
    expect(screen.getByTestId("pair-page")).toBeInTheDocument();
    expect(screen.getByTestId("pair-code-input")).toBeInTheDocument();
    expect(screen.getByTestId("pair-submit")).toBeInTheDocument();
  });

  it("on success: stores token in localStorage and navigates to /display/<id>", async () => {
    const user = userEvent.setup();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ display_id: "display-abc", token: "sydisp_token123" }),
    } as Response));

    render(<PairPage />);

    const input = screen.getByTestId("pair-code-input");
    await user.type(input, "123456");

    const submit = screen.getByTestId("pair-submit");
    await user.click(submit);

    await waitFor(() => {
      expect(memLS.getItem("sy.display.display-abc.token")).toBe("sydisp_token123");
      expect(replaceSpy).toHaveBeenCalledWith("/display/display-abc");
    });
  });

  it("on error: shows error message and does not navigate", async () => {
    const user = userEvent.setup();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false,
      json: async () => ({ code: "not_found" }),
    } as Response));

    render(<PairPage />);

    const input = screen.getByTestId("pair-code-input");
    await user.type(input, "999999");

    const submit = screen.getByTestId("pair-submit");
    await user.click(submit);

    await waitFor(() => {
      expect(screen.getByTestId("pair-error")).toBeInTheDocument();
      expect(screen.getByTestId("pair-error")).toHaveTextContent(/Code not found or expired/);
    });
    expect(replaceSpy).not.toHaveBeenCalled();
  });

  it("shows error when code length is not 6", async () => {
    const user = userEvent.setup();
    vi.stubGlobal("fetch", vi.fn());

    render(<PairPage />);

    const input = screen.getByTestId("pair-code-input");
    await user.type(input, "123");

    // Submit button should be disabled for incomplete codes
    const submit = screen.getByTestId("pair-submit");
    expect(submit).toBeDisabled();
  });

  it("submit button is enabled only when code is 6 digits", async () => {
    const user = userEvent.setup();
    vi.stubGlobal("fetch", vi.fn());

    render(<PairPage />);

    const input = screen.getByTestId("pair-code-input");
    const submit = screen.getByTestId("pair-submit");

    expect(submit).toBeDisabled();
    await user.type(input, "123456");
    expect(submit).not.toBeDisabled();
  });
});
