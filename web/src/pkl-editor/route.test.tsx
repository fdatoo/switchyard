import { test, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import PklEditorRoute from "./route";

// Mock Monaco so we don't load the real editor in tests
vi.mock("./Monaco", () => ({
  default: ({ value }: { value: string }) => (
    <textarea data-testid="editor" defaultValue={value} />
  ),
}));

vi.mock("../data/starlarkls-client", () => ({
  starlarkLsClient: {
    complete: vi.fn().mockResolvedValue({ items: [] }),
    hover: vi.fn().mockResolvedValue({ markdown: "" }),
    lookupSymbol: vi.fn().mockResolvedValue({ kind: "function" }),
  },
}));

test("renders FileTree and editor area", () => {
  render(<PklEditorRoute filePath="automations/sunset-lights.pkl" />);
  // Monaco (mocked) renders
  expect(screen.getByTestId("editor")).toBeInTheDocument();
});

test("renders status bar with apply button", () => {
  render(<PklEditorRoute filePath="automations/sunset-lights.pkl" />);
  expect(
    screen.getByRole("button", { name: /apply changes/i })
  ).toBeInTheDocument();
});
