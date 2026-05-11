import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import React from "react";
import { Palette } from "./palette";
import type { Verb } from "./palette-state";

// Mock use-mcp-configured to control the Ask section.
vi.mock("./use-mcp-configured", () => ({
  useMcpConfigured: vi.fn(() => false),
}));

// Mock recently-used to return predictable data.
vi.mock("./recently-used", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./recently-used")>();
  return {
    ...actual,
    useRecentlyUsed: vi.fn(() => [
      { verbName: "events tail", args: { source: "z2m" }, ranAt: new Date(Date.now() - 60_000).toISOString() },
    ]),
    usePaletteCliPreview: vi.fn(() => [false, vi.fn()] as [boolean, (on: boolean) => void]),
  };
});

const testVerbs: Verb[] = [
  {
    name: "events tail",
    description: "Stream events",
    cliForm: "switchyard event tail",
    handlerRef: "events.tail",
    args: [
      { name: "source", type: "string", required: false, cliFlag: "--source", hint: "driver name" },
      { name: "kind", type: "string", required: false, cliFlag: "--kind", hint: "" },
    ],
  },
  {
    name: "entity get",
    description: "Fetch entity",
    cliForm: "switchyard entity get <id>",
    handlerRef: "entity.get",
    args: [
      { name: "id", type: "string", required: true, cliFlag: "--id", hint: "entity id" },
    ],
  },
];

describe("Palette", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders RECENTLY USED and JUMP TO sections in default state", () => {
    render(
      <Palette
        open={true}
        onClose={vi.fn()}
        catalog={testVerbs}
      />,
    );
    expect(screen.getByText("RECENTLY USED")).toBeInTheDocument();
    expect(screen.getByText("JUMP TO")).toBeInTheDocument();
  });

  it("does not render Ask section when MCP is not configured", async () => {
    render(
      <Palette
        open={true}
        onClose={vi.fn()}
        catalog={testVerbs}
      />,
    );
    expect(screen.queryByText(/Ask the Switchyard agent/i)).not.toBeInTheDocument();
  });

  it("renders Ask section when MCP is configured", async () => {
    const { useMcpConfigured } = await import("./use-mcp-configured");
    vi.mocked(useMcpConfigured).mockReturnValue(true);

    render(
      <Palette
        open={true}
        onClose={vi.fn()}
        catalog={testVerbs}
      />,
    );
    expect(screen.getByText(/Ask the Switchyard agent/i)).toBeInTheDocument();
  });

  it("renders verb chip and arg chip when input resolves", () => {
    render(
      <Palette
        open={true}
        onClose={vi.fn()}
        catalog={testVerbs}
      />,
    );
    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "tail z2m" } });
    // The resolved-as row should show the verb name and arg.
    expect(screen.getByTestId("palette-resolved-verb")).toBeInTheDocument();
    expect(screen.getByTestId("palette-resolved-verb").textContent).toContain("events tail");
  });

  it("shows CLI preview string when cliPreview is on", async () => {
    const { usePaletteCliPreview } = await import("./recently-used");
    vi.mocked(usePaletteCliPreview).mockReturnValue([true, vi.fn()]);

    render(
      <Palette
        open={true}
        onClose={vi.fn()}
        catalog={testVerbs}
      />,
    );
    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "tail source:z2m" } });
    expect(screen.getByTestId("palette-cli-preview")).toBeInTheDocument();
    expect(screen.getByTestId("palette-cli-preview").textContent).toContain(
      "switchyard event tail",
    );
  });

  it("hides CLI preview string when cliPreview is off", async () => {
    const { usePaletteCliPreview } = await import("./recently-used");
    vi.mocked(usePaletteCliPreview).mockReturnValue([false, vi.fn()]);

    render(
      <Palette
        open={true}
        onClose={vi.fn()}
        catalog={testVerbs}
      />,
    );
    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "tail source:z2m" } });
    expect(screen.queryByTestId("palette-cli-preview")).not.toBeInTheDocument();
  });

  it("closes on Escape key", () => {
    const onClose = vi.fn();
    render(
      <Palette
        open={true}
        onClose={onClose}
        catalog={testVerbs}
      />,
    );
    const input = screen.getByRole("textbox");
    fireEvent.keyDown(input, { key: "Escape" });
    expect(onClose).toHaveBeenCalled();
  });
});
