import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, test, vi, expect } from "vitest";
import { LanguageProvider } from "../../language-provider";
import { LanguagePrimitives, usePrimitive } from "../../primitives-provider";

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

function SurfaceConsumer() {
  const Surface = usePrimitive("Surface");
  return <Surface data-testid="surface">content</Surface>;
}

test("developer Surface renders with data-variant=developer-surface", () => {
  render(
    <LanguageProvider initialLanguage="developer">
      <LanguagePrimitives>
        <SurfaceConsumer />
      </LanguagePrimitives>
    </LanguageProvider>,
  );
  expect(screen.getByTestId("surface")).toHaveAttribute(
    "data-variant",
    "developer-surface",
  );
});

test("developer Surface renders children", () => {
  render(
    <LanguageProvider initialLanguage="developer">
      <LanguagePrimitives>
        <SurfaceConsumer />
      </LanguagePrimitives>
    </LanguageProvider>,
  );
  expect(screen.getByTestId("surface")).toHaveTextContent("content");
});
