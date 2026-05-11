import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { DoSection } from "./DoSection";
import type { ActionDraft } from "../useAutomationEditor";

describe("DoSection", () => {
  it("renders entity picker for TurnOn action", () => {
    // Entity picker is a select; the entity value should match one of the mock entities
    const actions: ActionDraft[] = [{ type: "TurnOn", entity: "light.kitchen" }];
    render(<DoSection actions={actions} onChange={() => undefined} />);
    // Entity picker is a select with the entity value
    expect(screen.getByDisplayValue("light.kitchen")).toBeDefined();
  });

  it("renders brightness slider for SetBrightness action", () => {
    const actions: ActionDraft[] = [{ type: "SetBrightness", entity: "light.kitchen", brightness: 40 }];
    render(<DoSection actions={actions} onChange={() => undefined} />);
    // Slider should be present with the correct value
    const slider = screen.getByRole("slider");
    expect(slider).toBeDefined();
    expect((slider as HTMLInputElement).value).toBe("40");
  });

  it("renders 'starlark' chip and 'View in Pkl editor →' for Starlark action", () => {
    const actions: ActionDraft[] = [
      {
        type: "Starlark",
        starlarkBody: "print('hello')",
        starlarkFilePath: "automations/test.pkl",
        starlarkLine: 5,
      },
    ];
    render(<DoSection actions={actions} onChange={() => undefined} />);
    expect(screen.getByText("starlark")).toBeDefined();
    const link = screen.getByRole("link", { name: /view in pkl editor/i });
    expect(link).toBeDefined();
  });

  it("shows 'No actions defined yet.' for empty actions", () => {
    render(<DoSection actions={[]} onChange={() => undefined} />);
    expect(screen.getByText(/no actions defined yet/i)).toBeDefined();
  });
});
