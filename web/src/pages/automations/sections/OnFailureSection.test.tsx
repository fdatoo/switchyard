import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { OnFailureSection } from "./OnFailureSection";

describe("OnFailureSection", () => {
  it("shows no extra fields for 'ignore' strategy", () => {
    render(
      <OnFailureSection onFailure={{ strategy: "ignore" }} onChange={() => undefined} />,
    );
    expect(screen.queryByLabelText(/max attempts/i)).toBeNull();
    expect(screen.queryByLabelText(/backoff/i)).toBeNull();
  });

  it("shows maxAttempts and backoff inputs for 'retry' strategy", () => {
    render(
      <OnFailureSection onFailure={{ strategy: "retry", maxAttempts: 3 }} onChange={() => undefined} />,
    );
    expect(screen.getByLabelText(/max attempts/i)).toBeDefined();
    expect(screen.getByLabelText(/backoff/i)).toBeDefined();
  });

  it("fires onChange with updated maxAttempts when changed", () => {
    const handleChange = vi.fn();
    render(
      <OnFailureSection
        onFailure={{ strategy: "retry", maxAttempts: 3 }}
        onChange={handleChange}
      />,
    );
    const maxAttemptsInput = screen.getByLabelText(/max attempts/i);
    fireEvent.change(maxAttemptsInput, { target: { value: "5" } });
    expect(handleChange).toHaveBeenCalledWith(
      expect.objectContaining({ maxAttempts: 5 }),
    );
  });

  it("shows notify fields for 'notify' strategy", () => {
    render(
      <OnFailureSection
        onFailure={{ strategy: "notify", entity: "notify.phone" }}
        onChange={() => undefined}
      />,
    );
    expect(screen.getByDisplayValue("notify.phone")).toBeDefined();
  });
});
