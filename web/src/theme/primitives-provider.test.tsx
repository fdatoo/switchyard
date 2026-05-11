import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { LanguageProvider } from "./language-provider";
import { LanguagePrimitives, usePrimitive } from "./primitives-provider";
import { Surface } from "./primitives/surface";

function makeLsStub(language?: string): Storage {
  const stored = language ? JSON.stringify({ language, mode: "system" }) : null;
  return {
    getItem: (k: string) => (k === "sy.theme.v2" ? stored : null),
    setItem: () => undefined,
    removeItem: () => undefined,
    clear: () => undefined,
    length: language ? 1 : 0,
    key: () => null,
  } satisfies Storage;
}

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

// Wrapper that uses the stored language (friendly by default)
function Wrapper({ children }: { children: ReactNode }) {
  return (
    <LanguageProvider>
      <LanguagePrimitives>{children}</LanguagePrimitives>
    </LanguageProvider>
  );
}

// Consumer that renders the Surface primitive from the registry
function SurfaceConsumer({ testId }: { testId: string }) {
  const SurfaceComponent = usePrimitive("Surface");
  return <SurfaceComponent data-testid={testId}>content</SurfaceComponent>;
}

describe("LanguagePrimitives provider", () => {
  beforeEach(() => {
    vi.stubGlobal("localStorage", makeLsStub());
    vi.stubGlobal("matchMedia", makeMatchMediaStub());
    delete document.documentElement.dataset.theme;
    delete document.documentElement.dataset.language;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    delete document.documentElement.dataset.theme;
    delete document.documentElement.dataset.language;
  });

  it("renders the friendly Surface when language=friendly", () => {
    render(
      <Wrapper>
        <SurfaceConsumer testId="surface-friendly" />
      </Wrapper>,
    );
    expect(screen.getByTestId("surface-friendly")).toBeInTheDocument();
    expect(screen.getByTestId("surface-friendly")).toHaveTextContent("content");
  });

  it("renders the direct <Surface> component without provider", () => {
    render(<Surface data-testid="surface-direct">hello</Surface>);
    expect(screen.getByTestId("surface-direct")).toBeInTheDocument();
    expect(screen.getByTestId("surface-direct")).toHaveTextContent("hello");
  });

  it("renders developer Surface variant with data-variant=developer-surface", () => {
    // Override localStorage to return developer before rendering
    vi.stubGlobal("localStorage", makeLsStub("developer"));

    render(
      <Wrapper>
        <SurfaceConsumer testId="surface-dev" />
      </Wrapper>,
    );

    // Developer now has a registered Surface variant
    const el = screen.getByTestId("surface-dev");
    expect(el).toBeInTheDocument();
    expect(el).toHaveAttribute("data-variant", "developer-surface");
    expect(el).toHaveTextContent("content");
  });

  it("friendly language: Button renders without developer variant attribute", () => {
    function ButtonConsumer() {
      const Button = usePrimitive("Button");
      return <Button data-testid="btn">click</Button>;
    }
    render(
      <LanguageProvider initialLanguage="friendly">
        <LanguagePrimitives>
          <ButtonConsumer />
        </LanguagePrimitives>
      </LanguageProvider>,
    );
    expect(screen.getByTestId("btn")).not.toHaveAttribute(
      "data-variant",
      "developer-button",
    );
  });

  it("developer language: Button renders with data-variant=developer-button", () => {
    function ButtonConsumer() {
      const Button = usePrimitive("Button");
      return <Button data-testid="btn">click</Button>;
    }
    render(
      <LanguageProvider initialLanguage="developer">
        <LanguagePrimitives>
          <ButtonConsumer />
        </LanguagePrimitives>
      </LanguageProvider>,
    );
    expect(screen.getByTestId("btn")).toHaveAttribute(
      "data-variant",
      "developer-button",
    );
  });

  it("developer language: Chip renders with data-variant=developer-chip", () => {
    function ChipConsumer() {
      const Chip = usePrimitive("Chip");
      return <Chip data-testid="chip">label</Chip>;
    }
    render(
      <LanguageProvider initialLanguage="developer">
        <LanguagePrimitives>
          <ChipConsumer />
        </LanguagePrimitives>
      </LanguageProvider>,
    );
    expect(screen.getByTestId("chip")).toHaveAttribute(
      "data-variant",
      "developer-chip",
    );
  });

  it("developer language: Pill renders with data-variant=developer-pill", () => {
    function PillConsumer() {
      const Pill = usePrimitive("Pill");
      return <Pill data-testid="pill">status</Pill>;
    }
    render(
      <LanguageProvider initialLanguage="developer">
        <LanguagePrimitives>
          <PillConsumer />
        </LanguagePrimitives>
      </LanguageProvider>,
    );
    expect(screen.getByTestId("pill")).toHaveAttribute(
      "data-variant",
      "developer-pill",
    );
  });
});
