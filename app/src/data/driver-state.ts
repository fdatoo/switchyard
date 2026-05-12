/**
 * Display mapping for driver states.
 *
 * Centralized so views, panels, and any future driver-status surfaces all
 * agree on the same intent / pulse / label per state.
 */

import type { DriverStateName } from "./driver-management";

export type Intent = "good" | "warn" | "bad" | "neutral";
export type Pulse = "slow" | "fast" | "off";

interface StatePresentation {
  intent: Intent;
  pulse: Pulse;
  label: string;
}

const PRESENTATION: Record<DriverStateName, StatePresentation> = {
  running:      { intent: "good",    pulse: "slow", label: "Running" },
  reconnecting: { intent: "warn",    pulse: "fast", label: "Reconnecting" },
  degraded:     { intent: "warn",    pulse: "off",  label: "Degraded" },
  stopped:      { intent: "bad",     pulse: "off",  label: "Stopped" },
  unknown:      { intent: "neutral", pulse: "off",  label: "Unknown" },
};

export function presentDriverState(state: DriverStateName): StatePresentation {
  return PRESENTATION[state];
}
