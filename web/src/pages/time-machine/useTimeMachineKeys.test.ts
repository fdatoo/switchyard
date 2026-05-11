import { describe, it, expect, vi, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { fireEvent } from "@testing-library/dom";
import { useTimeMachineKeys } from "./useTimeMachineKeys";

function makeHandlers(overrides: Partial<Parameters<typeof useTimeMachineKeys>[0]> = {}) {
  return {
    playing: false,
    onPlay: vi.fn(),
    onPause: vi.fn(),
    onStepForward: vi.fn(),
    onStepBack: vi.fn(),
    onJumpForward: vi.fn(),
    onJumpBack: vi.fn(),
    onToggleAffected: vi.fn(),
    onToggleDiff: vi.fn(),
    onExit: vi.fn(),
    ...overrides,
  };
}

afterEach(() => {
  // Ensure any active element is reset.
  (document.activeElement as HTMLElement | null)?.blur?.();
});

describe("useTimeMachineKeys", () => {
  it("ArrowRight fires onStepForward", () => {
    const handlers = makeHandlers();
    renderHook(() => useTimeMachineKeys(handlers));
    fireEvent.keyDown(window, { key: "ArrowRight" });
    expect(handlers.onStepForward).toHaveBeenCalledOnce();
    expect(handlers.onJumpForward).not.toHaveBeenCalled();
  });

  it("Shift+ArrowRight fires onJumpForward, not onStepForward", () => {
    const handlers = makeHandlers();
    renderHook(() => useTimeMachineKeys(handlers));
    fireEvent.keyDown(window, { key: "ArrowRight", shiftKey: true });
    expect(handlers.onJumpForward).toHaveBeenCalledOnce();
    expect(handlers.onStepForward).not.toHaveBeenCalled();
  });

  it("Space fires onPause when playing=true", () => {
    const handlers = makeHandlers({ playing: true });
    renderHook(() => useTimeMachineKeys(handlers));
    fireEvent.keyDown(window, { key: " " });
    expect(handlers.onPause).toHaveBeenCalledOnce();
    expect(handlers.onPlay).not.toHaveBeenCalled();
  });

  it("Space fires onPlay when playing=false", () => {
    const handlers = makeHandlers({ playing: false });
    renderHook(() => useTimeMachineKeys(handlers));
    fireEvent.keyDown(window, { key: " " });
    expect(handlers.onPlay).toHaveBeenCalledOnce();
    expect(handlers.onPause).not.toHaveBeenCalled();
  });

  it("no callback when document.activeElement is an input", () => {
    const handlers = makeHandlers();
    renderHook(() => useTimeMachineKeys(handlers));

    // Create and focus an input
    const input = document.createElement("input");
    document.body.appendChild(input);
    input.focus();

    fireEvent.keyDown(window, { key: "ArrowRight" });
    expect(handlers.onStepForward).not.toHaveBeenCalled();

    input.blur();
    document.body.removeChild(input);
  });

  it("f fires onToggleAffected", () => {
    const handlers = makeHandlers();
    renderHook(() => useTimeMachineKeys(handlers));
    fireEvent.keyDown(window, { key: "f" });
    expect(handlers.onToggleAffected).toHaveBeenCalledOnce();
  });

  it("d fires onToggleDiff", () => {
    const handlers = makeHandlers();
    renderHook(() => useTimeMachineKeys(handlers));
    fireEvent.keyDown(window, { key: "d" });
    expect(handlers.onToggleDiff).toHaveBeenCalledOnce();
  });

  it("Escape fires onExit", () => {
    const handlers = makeHandlers();
    renderHook(() => useTimeMachineKeys(handlers));
    fireEvent.keyDown(window, { key: "Escape" });
    expect(handlers.onExit).toHaveBeenCalledOnce();
  });
});
