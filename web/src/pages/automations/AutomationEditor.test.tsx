import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { AutomationEditor } from "./AutomationEditor";

// Mock useAutomationEditor to avoid real edit session calls
vi.mock("./useAutomationEditor", () => ({
  useAutomationEditor: vi.fn(),
}));

import { useAutomationEditor } from "./useAutomationEditor";

const baseReturn = {
  editorState: {
    id: "sunset-lights",
    displayName: "Sunset Lights",
    enabled: true,
    trigger: { type: "SunEvent" as const, sunEvent: "sunset" as const },
    multipleTriggersDetected: false,
    conditions: [],
    actions: [],
    onFailure: { strategy: "ignore" as const },
  },
  pklSource: `import "switchyard:automations" as auto\nnew auto.Automation { id = "sunset-lights" }`,
  isDirty: false,
  conflict: null,
  sessionStatus: "open",
  sessionError: null,
  updateTrigger: vi.fn(),
  updateConditions: vi.fn(),
  updateActions: vi.fn(),
  updateOnFailure: vi.fn(),
  save: vi.fn().mockResolvedValue(undefined),
  discard: vi.fn().mockResolvedValue(undefined),
  resolveForce: vi.fn().mockResolvedValue(undefined),
  resolveOpenMerge: vi.fn(),
};

beforeEach(() => {
  vi.mocked(useAutomationEditor).mockReturnValue(baseReturn);
});

describe("AutomationEditor", () => {
  it("renders all four section card headings", () => {
    render(<AutomationEditor slug="sunset-lights" />);
    expect(screen.getByText("Trigger")).toBeDefined();
    expect(screen.getByText("When")).toBeDefined();
    expect(screen.getByText("Do")).toBeDefined();
    expect(screen.getByText("On failure")).toBeDefined();
  });

  it("disables Save & exit when not dirty", () => {
    render(<AutomationEditor slug="sunset-lights" />);
    const saveBtn = screen.getByRole("button", { name: /save.*exit/i });
    expect(saveBtn).toBeDefined();
    expect((saveBtn as HTMLButtonElement).disabled).toBe(true);
  });

  it("enables Save & exit when dirty", () => {
    vi.mocked(useAutomationEditor).mockReturnValue({ ...baseReturn, isDirty: true });
    render(<AutomationEditor slug="sunset-lights" />);
    const saveBtn = screen.getByRole("button", { name: /save.*exit/i });
    expect((saveBtn as HTMLButtonElement).disabled).toBe(false);
  });

  it("renders ConflictBanner when conflict is non-null", () => {
    vi.mocked(useAutomationEditor).mockReturnValue({
      ...baseReturn,
      conflict: {
        diskHash: "abc",
        diskPkl: "...",
        ancestorPkl: "...",
        stagedPkl: "...",
      },
    });
    render(<AutomationEditor slug="sunset-lights" />);
    // ConflictBanner has role="alert"
    expect(screen.getByRole("alert")).toBeDefined();
  });

  it("renders the PklSourcePane", () => {
    render(<AutomationEditor slug="sunset-lights" />);
    expect(screen.getByRole("complementary", { name: /pkl source pane/i })).toBeDefined();
  });
});
