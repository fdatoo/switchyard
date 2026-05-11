import { test, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import MergeRoute from "./merge-route";

vi.mock("./Monaco", () => ({
  default: ({
    value,
    "data-testid": testId,
  }: {
    value: string;
    "data-testid"?: string;
  }) => <textarea data-testid={testId ?? "editor"} defaultValue={value} />,
}));

// Mock window.location for the test
Object.defineProperty(window, "location", {
  value: {
    pathname: "/_authed/pkl-editor/merge/automations/sunset-lights.pkl",
    search: "?session=abc",
    href: "http://localhost/_authed/pkl-editor/merge/automations/sunset-lights.pkl?session=abc",
  },
  writable: true,
});

test("renders three editor panes with correct labels", () => {
  render(<MergeRoute />);
  // Use getAllByText to handle multiple matches (headers + editor content stubs)
  expect(screen.getAllByText(/on disk now/i).length).toBeGreaterThan(0);
  expect(screen.getAllByText(/common ancestor/i).length).toBeGreaterThan(0);
  expect(screen.getAllByText(/your changes/i).length).toBeGreaterThan(0);
});
