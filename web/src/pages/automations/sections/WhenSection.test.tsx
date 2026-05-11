import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { WhenSection } from "./WhenSection";
import type { ConditionDraft } from "../useAutomationEditor";

describe("WhenSection", () => {
  it("shows empty state when conditions is empty", () => {
    render(<WhenSection conditions={[]} onChange={() => undefined} />);
    expect(screen.getByText(/no conditions/i)).toBeDefined();
  });

  it("renders a StateCondition leaf with entity name", () => {
    const conditions: ConditionDraft[] = [
      { type: "StateEq", entity: "light.living_room", value: "on" },
    ];
    render(<WhenSection conditions={conditions} onChange={() => undefined} />);
    const entityInput = screen.getByDisplayValue("light.living_room");
    expect(entityInput).toBeDefined();
  });

  it("renders Starlark leaf with 'View in Pkl editor →' link", () => {
    const conditions: ConditionDraft[] = [
      {
        type: "Starlark",
        starlarkExpr: `state("sensor.lux").value < 50`,
        starlarkFilePath: "automations/sunset-lights.pkl",
        starlarkLine: 12,
      },
    ];
    render(<WhenSection conditions={conditions} onChange={() => undefined} />);
    const link = screen.getByRole("link", { name: /view in pkl editor/i });
    expect(link).toBeDefined();
    expect(link.getAttribute("href")).toContain("sunset-lights.pkl");
  });

  it("fires onChange with one-element array when removing first of two conditions", () => {
    const handleChange = vi.fn();
    const conditions: ConditionDraft[] = [
      { type: "StateEq", entity: "light.a", value: "on" },
      { type: "StateEq", entity: "light.b", value: "off" },
    ];
    render(<WhenSection conditions={conditions} onChange={handleChange} />);
    const removeButtons = screen.getAllByRole("button", { name: /remove condition/i });
    fireEvent.click(removeButtons[0]);
    expect(handleChange).toHaveBeenCalledWith([conditions[1]]);
  });
});
