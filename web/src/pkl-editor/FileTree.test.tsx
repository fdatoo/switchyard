import { test, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import FileTree from "./FileTree";

const FILES = [
  { path: "automations/sunset-lights.pkl", dirty: true, hasError: false },
  { path: "automations/morning.pkl", dirty: false, hasError: true },
  { path: "base/main.pkl", dirty: false, hasError: false },
];

test("renders dirty dot for dirty files", () => {
  render(<FileTree files={FILES} activePath="" onSelect={() => {}} onSearch={() => {}} />);
  const dots = screen.getAllByRole("status");
  expect(dots.some((d) => d.getAttribute("aria-label") === "unsaved changes")).toBe(true);
});

test("renders error badge for files with errors", () => {
  render(<FileTree files={FILES} activePath="" onSelect={() => {}} onSearch={() => {}} />);
  expect(screen.getByLabelText("has errors")).toBeInTheDocument();
});

test("calls onSelect with full path on file click", () => {
  const onSelect = vi.fn();
  render(<FileTree files={FILES} activePath="" onSelect={onSelect} onSearch={() => {}} />);
  fireEvent.click(screen.getByText("sunset-lights.pkl"));
  expect(onSelect).toHaveBeenCalledWith("automations/sunset-lights.pkl");
});

test("calls onSearch when search input is activated", () => {
  const onSearch = vi.fn();
  render(<FileTree files={FILES} activePath="" onSelect={() => {}} onSearch={onSearch} />);
  fireEvent.click(screen.getByPlaceholderText(/find file/i));
  expect(onSearch).toHaveBeenCalled();
});
