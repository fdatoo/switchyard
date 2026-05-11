import { test, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Suspense } from "react";
import Monaco from "./Monaco";

test("renders loading fallback before Monaco resolves", () => {
  render(
    <Suspense fallback={<div data-testid="loading">Loading editor…</div>}>
      <Monaco language="pkl" value="" onChange={() => {}} />
    </Suspense>
  );
  // Monaco renders its own internal fallback ("editor-loading") while monaco-editor loads.
  // In jsdom the dynamic import doesn't resolve synchronously so we see the fallback.
  expect(screen.getByTestId("editor-loading")).toBeInTheDocument();
});
