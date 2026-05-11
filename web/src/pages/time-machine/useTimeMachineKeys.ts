import { useEffect } from "react";

export interface TimeMachineKeyHandlers {
  playing: boolean;
  onPlay: () => void;
  onPause: () => void;
  onStepForward: () => void;
  onStepBack: () => void;
  onJumpForward: () => void;
  onJumpBack: () => void;
  onToggleAffected: () => void;
  onToggleDiff: () => void;
  onExit: () => void;
}

/** Returns true if focus is in a form control (suppress shortcuts). */
function isFocusInFormControl(): boolean {
  const el = document.activeElement;
  if (!el) return false;
  const tag = el.tagName.toUpperCase();
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
  if ((el as HTMLElement).isContentEditable) return true;
  return false;
}

/**
 * useTimeMachineKeys attaches keyboard shortcuts for time-machine navigation.
 * All shortcuts are suppressed when focus is in a form control.
 *
 * Space     → play/pause toggle
 * ArrowRight → step forward
 * ArrowLeft  → step back
 * Shift+ArrowRight → jump 1s forward
 * Shift+ArrowLeft  → jump 1s back
 * f → toggle affected-only mode
 * d → toggle diff mode
 * Escape → exit time machine
 */
export function useTimeMachineKeys({
  playing,
  onPlay,
  onPause,
  onStepForward,
  onStepBack,
  onJumpForward,
  onJumpBack,
  onToggleAffected,
  onToggleDiff,
  onExit,
}: TimeMachineKeyHandlers): void {
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if (isFocusInFormControl()) return;

      switch (event.key) {
        case " ":
          event.preventDefault();
          if (playing) {
            onPause();
          } else {
            onPlay();
          }
          break;
        case "ArrowRight":
          event.preventDefault();
          if (event.shiftKey) {
            onJumpForward();
          } else {
            onStepForward();
          }
          break;
        case "ArrowLeft":
          event.preventDefault();
          if (event.shiftKey) {
            onJumpBack();
          } else {
            onStepBack();
          }
          break;
        case "f":
          event.preventDefault();
          onToggleAffected();
          break;
        case "d":
          event.preventDefault();
          onToggleDiff();
          break;
        case "Escape":
          event.preventDefault();
          onExit();
          break;
      }
    };

    window.addEventListener("keydown", handler);
    return () => {
      window.removeEventListener("keydown", handler);
    };
  }, [playing, onPlay, onPause, onStepForward, onStepBack, onJumpForward, onJumpBack, onToggleAffected, onToggleDiff, onExit]);
}
