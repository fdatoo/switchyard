import { test, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import AstBreadcrumb from "./AstBreadcrumb";

test("renders path segments as breadcrumb items", () => {
  render(<AstBreadcrumb path={["automations", "sunset-lights.pkl", "actions [2]", "brightness"]} />);
  expect(screen.getByText("automations")).toBeInTheDocument();
  expect(screen.getByText("brightness")).toBeInTheDocument();
});

test("renders empty state when path is empty", () => {
  const { container } = render(<AstBreadcrumb path={[]} />);
  expect(container.querySelector("nav")?.children.length).toBe(0);
});
