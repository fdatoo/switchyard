import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { TopBar } from "./TopBar";
import { LanguageProvider } from "../theme/language-provider";

// Mock the palette hook so TopBar tests don't need the full PaletteProvider stack.
vi.mock("@/palette/use-palette", () => ({
  usePalette: () => ({
    openPalette: vi.fn(),
    closePalette: vi.fn(),
    isOpen: false,
  }),
}));

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

describe("TopBar", () => {
  it("renders the breadcrumb with the current page name derived from the path", () => {
    render(<TopBar currentPath="/_authed/activity" />);

    const breadcrumb = screen.getByRole("navigation", { name: /breadcrumb/i });
    expect(breadcrumb).toBeInTheDocument();
    expect(breadcrumb).toHaveTextContent("Activity");
  });

  it("renders Home when the path is /_authed/home", () => {
    render(<TopBar currentPath="/_authed/home" />);
    expect(screen.getByRole("navigation", { name: /breadcrumb/i })).toHaveTextContent("Home");
  });

  it("renders the command palette button", () => {
    render(<TopBar currentPath="/_authed/home" />);
    expect(screen.getByRole("button", { name: /open command palette/i })).toBeInTheDocument();
  });

  it("developer language: breadcrumb reads 'Events' at /activity", () => {
    render(
      <LanguageProvider initialLanguage="developer">
        <TopBar currentPath="/_authed/activity" />
      </LanguageProvider>,
    );
    expect(screen.getByRole("navigation", { name: /breadcrumb/i })).toHaveTextContent("Events");
  });

  it("developer language: breadcrumb reads 'Overview' at /home", () => {
    render(
      <LanguageProvider initialLanguage="developer">
        <TopBar currentPath="/_authed/home" />
      </LanguageProvider>,
    );
    expect(screen.getByRole("navigation", { name: /breadcrumb/i })).toHaveTextContent("Overview");
  });

  it("nested path: breadcrumb reads 'Settings › Drivers' at /settings/drivers", () => {
    render(<TopBar currentPath="/_authed/settings/drivers" />);
    const breadcrumb = screen.getByRole("navigation", { name: /breadcrumb/i });
    expect(breadcrumb).toHaveTextContent("Settings");
    expect(breadcrumb).toHaveTextContent("Drivers");
  });

  it("room slug: breadcrumb reads 'Rooms › My Room' at /rooms/my-room", () => {
    render(<TopBar currentPath="/_authed/rooms/my-room" />);
    const breadcrumb = screen.getByRole("navigation", { name: /breadcrumb/i });
    expect(breadcrumb).toHaveTextContent("Rooms");
    expect(breadcrumb).toHaveTextContent("My Room");
  });

  it("special settings sections: breadcrumb reads 'Settings › Pkl config' at /settings/pkl-config", () => {
    render(<TopBar currentPath="/_authed/settings/pkl-config" />);
    const breadcrumb = screen.getByRole("navigation", { name: /breadcrumb/i });
    expect(breadcrumb).toHaveTextContent("Settings");
    expect(breadcrumb).toHaveTextContent("Pkl config");
  });

  it("displays: breadcrumb reads 'Displays' at /displays", () => {
    render(<TopBar currentPath="/_authed/displays" />);
    expect(screen.getByRole("navigation", { name: /breadcrumb/i })).toHaveTextContent("Displays");
  });

  it("ask page: breadcrumb reads 'Ask'", () => {
    render(<TopBar currentPath="/_authed/ask" />);
    expect(screen.getByRole("navigation", { name: /breadcrumb/i })).toHaveTextContent("Ask");
  });
});
