import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { LanguageProvider } from "../../../theme/language-provider";
import { RoomGridSection } from "./RoomGrid";
import type { SectionDef } from "../../model";

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

const sectionDef: SectionDef = {
  id: "rooms-section",
  type: "RoomGrid",
  props: { title: "Rooms" },
  tiles: [
    {
      id: "r1",
      type: "RoomTile",
      props: { name: "Living Room", state: "on", scene: "Evening", brightness: 72, sinceMs: 120_000 },
    },
  ],
};

test("renders RoomsTable (table element) when language=developer", () => {
  render(
    <LanguageProvider initialLanguage="developer">
      <RoomGridSection def={sectionDef} />
    </LanguageProvider>,
  );
  expect(screen.getByRole("table")).toBeInTheDocument();
});

test("does not render a table when language=friendly", () => {
  render(
    <LanguageProvider initialLanguage="friendly">
      <RoomGridSection def={sectionDef} />
    </LanguageProvider>,
  );
  expect(screen.queryByRole("table")).not.toBeInTheDocument();
});

test("does not render a table when language=ambient", () => {
  render(
    <LanguageProvider initialLanguage="ambient">
      <RoomGridSection def={sectionDef} />
    </LanguageProvider>,
  );
  expect(screen.queryByRole("table")).not.toBeInTheDocument();
});
