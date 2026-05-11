import { render, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider, useLanguage } from "./language-provider";

// Build an in-memory localStorage stub
function makeLocalStorageStub() {
  const store: Record<string, string> = {};
  return {
    getItem: (k: string) => store[k] ?? null,
    setItem: (k: string, v: string) => { store[k] = v; },
    removeItem: (k: string) => { delete store[k]; },
    clear: () => { for (const k of Object.keys(store)) delete store[k]; },
    get length() { return Object.keys(store).length; },
    key: (i: number) => Object.keys(store)[i] ?? null,
  } satisfies Storage;
}

let localStorageStub: Storage;

// Test component that reads from the context
function ReadTheme() {
  const { resolvedTheme, language, mode } = useLanguage();
  return (
    <div data-testid="out" data-resolved={resolvedTheme} data-language={language} data-mode={mode} />
  );
}

describe("LanguageProvider", () => {
  beforeEach(() => {
    localStorageStub = makeLocalStorageStub();
    vi.stubGlobal("localStorage", localStorageStub);

    // Reset documentElement data attributes
    delete document.documentElement.dataset.theme;
    delete document.documentElement.dataset.language;

    // Reset matchMedia stub to return light
    vi.stubGlobal("matchMedia", (query: string): MediaQueryList => ({
      matches: false, // prefers-color-scheme: light
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    }));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    delete document.documentElement.dataset.theme;
    delete document.documentElement.dataset.language;
  });

  it("sets data-theme=friendly-light when no localStorage and system prefers light", () => {
    render(
      <LanguageProvider>
        <ReadTheme />
      </LanguageProvider>,
    );
    expect(document.documentElement.dataset.theme).toBe("friendly-light");
    expect(document.documentElement.dataset.language).toBe("friendly");
  });

  it("calling setMode(dark) updates data-theme to friendly-dark", () => {
    let ctx: ReturnType<typeof useLanguage>;

    function Capture() {
      ctx = useLanguage();
      return null;
    }

    render(
      <LanguageProvider>
        <Capture />
      </LanguageProvider>,
    );

    act(() => {
      ctx!.setMode("dark");
    });

    expect(document.documentElement.dataset.theme).toBe("friendly-dark");
  });

  it("calling setLanguage(developer) sets data-theme=developer regardless of mode", () => {
    let ctx: ReturnType<typeof useLanguage>;

    function Capture() {
      ctx = useLanguage();
      return null;
    }

    render(
      <LanguageProvider>
        <Capture />
      </LanguageProvider>,
    );

    act(() => {
      ctx!.setLanguage("developer");
    });

    expect(document.documentElement.dataset.theme).toBe("developer");
  });

  it("persists and restores from localStorage on re-mount", () => {
    // First mount: switch to dark
    let ctx: ReturnType<typeof useLanguage>;

    function Capture() {
      ctx = useLanguage();
      return null;
    }

    const { unmount } = render(
      <LanguageProvider>
        <Capture />
      </LanguageProvider>,
    );

    act(() => {
      ctx!.setMode("dark");
    });

    unmount();

    // Reset documentElement (simulate page re-visit)
    delete document.documentElement.dataset.theme;
    delete document.documentElement.dataset.language;

    // Second mount: should restore dark from the stub localStorage
    render(
      <LanguageProvider>
        <ReadTheme />
      </LanguageProvider>,
    );

    expect(document.documentElement.dataset.theme).toBe("friendly-dark");
  });
});
