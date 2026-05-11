import { test, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import StatusBar from "./StatusBar";

test("shows unsaved count and error count", () => {
  render(
    <StatusBar
      pklVersion="0.27"
      unsavedCount={3}
      errorCount={1}
      formBoundCount={1}
      line={18}
      col={60}
      onFormat={vi.fn()}
      onValidate={vi.fn()}
      onApply={vi.fn()}
    />
  );
  expect(screen.getByText(/3 unsaved/)).toBeInTheDocument();
  expect(screen.getByText(/1 error/)).toBeInTheDocument();
});

test("calls onApply when Apply changes is clicked", () => {
  const onApply = vi.fn();
  render(
    <StatusBar pklVersion="0.27" unsavedCount={1} errorCount={0} formBoundCount={0}
      line={1} col={1} onFormat={vi.fn()} onValidate={vi.fn()} onApply={onApply} />
  );
  fireEvent.click(screen.getByRole("button", { name: /apply changes/i }));
  expect(onApply).toHaveBeenCalledTimes(1);
});
