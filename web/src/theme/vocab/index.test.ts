import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { LanguageProvider } from "../language-provider";
import { useVocab } from "./index";
import React from "react";

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

function makeLsStub(): Storage {
  return {
    getItem: () => null,
    setItem: () => undefined,
    removeItem: () => undefined,
    clear: () => undefined,
    length: 0,
    key: () => null,
  } satisfies Storage;
}

beforeEach(() => {
  vi.stubGlobal("localStorage", makeLsStub());
  vi.stubGlobal("matchMedia", makeMatchMediaStub());
});

afterEach(() => {
  vi.unstubAllGlobals();
  delete document.documentElement.dataset.theme;
  delete document.documentElement.dataset.language;
});

function wrapper(language: "friendly" | "developer" | "ambient") {
  return function TestWrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(LanguageProvider, { initialLanguage: language, children });
  };
}

test("friendly vocab: home route returns 'Home'", () => {
  const { result } = renderHook(() => useVocab(), {
    wrapper: wrapper("friendly"),
  });
  expect(result.current.label("home")).toBe("Home");
});

test("friendly vocab: rooms route returns 'Rooms'", () => {
  const { result } = renderHook(() => useVocab(), {
    wrapper: wrapper("friendly"),
  });
  expect(result.current.label("rooms")).toBe("Rooms");
});

test("developer vocab: home route returns 'Overview'", () => {
  const { result } = renderHook(() => useVocab(), {
    wrapper: wrapper("developer"),
  });
  expect(result.current.label("home")).toBe("Overview");
});

test("developer vocab: rooms route returns 'Entities'", () => {
  const { result } = renderHook(() => useVocab(), {
    wrapper: wrapper("developer"),
  });
  expect(result.current.label("rooms")).toBe("Entities");
});

test("developer vocab: activity route returns 'Events'", () => {
  const { result } = renderHook(() => useVocab(), {
    wrapper: wrapper("developer"),
  });
  expect(result.current.label("activity")).toBe("Events");
});

test("ambient vocab: home route returns 'Home'", () => {
  const { result } = renderHook(() => useVocab(), {
    wrapper: wrapper("ambient"),
  });
  expect(result.current.label("home")).toBe("Home");
});

test("all six route IDs return a non-empty string in every language", () => {
  const routeIds = [
    "home",
    "rooms",
    "activity",
    "automations",
    "devices",
    "settings",
  ] as const;
  const languages = ["friendly", "developer", "ambient"] as const;

  for (const language of languages) {
    const { result } = renderHook(() => useVocab(), {
      wrapper: wrapper(language),
    });
    for (const routeId of routeIds) {
      const label = result.current.label(routeId);
      expect(typeof label).toBe("string");
      expect(label.length).toBeGreaterThan(0);
    }
  }
});

test("friendly and developer differ for home and rooms", () => {
  const { result: friendlyResult } = renderHook(() => useVocab(), {
    wrapper: wrapper("friendly"),
  });
  const { result: developerResult } = renderHook(() => useVocab(), {
    wrapper: wrapper("developer"),
  });
  expect(friendlyResult.current.label("home")).not.toBe(
    developerResult.current.label("home"),
  );
  expect(friendlyResult.current.label("rooms")).not.toBe(
    developerResult.current.label("rooms"),
  );
});
