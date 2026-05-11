import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { TriggerSection } from "./TriggerSection";

describe("TriggerSection", () => {
  it("renders time input for Time trigger", () => {
    const trigger = { type: "Time" as const, timeAt: "21:30" };
    render(<TriggerSection trigger={trigger} onChange={() => undefined} />);
    const timeInput = screen.getByDisplayValue("21:30");
    expect(timeInput).toBeDefined();
  });

  it("renders read-only note for Manual trigger", () => {
    const trigger = { type: "Manual" as const };
    render(<TriggerSection trigger={trigger} onChange={() => undefined} />);
    expect(screen.getByText(/runs only when triggered manually/i)).toBeDefined();
  });

  it("fires onChange when trigger type changes to Webhook", () => {
    const handleChange = vi.fn();
    const trigger = { type: "Manual" as const };
    render(<TriggerSection trigger={trigger} onChange={handleChange} />);
    const select = screen.getByDisplayValue("Manual");
    fireEvent.change(select, { target: { value: "Webhook" } });
    expect(handleChange).toHaveBeenCalledWith({ type: "Webhook" });
  });

  it("renders multiple-triggers banner when multipleTriggersDetected", () => {
    render(
      <TriggerSection
        trigger={null}
        onChange={() => undefined}
        multipleTriggersDetected={true}
      />,
    );
    expect(screen.getByRole("note", { name: /multiple triggers/i })).toBeDefined();
  });
});
