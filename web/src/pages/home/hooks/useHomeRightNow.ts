// TODO(plan-03): replace with real EntityService + sensor/state data

export interface StatTile {
  id: string;
  label: string;
  value: string;
  unit: string;
  sublabel: string;
}

/**
 * useHomeRightNow — returns four stat tiles for the Right Now strip.
 * Mock data; Plan 03 will wire this to live entity state.
 */
export function useHomeRightNow(): StatTile[] {
  return [
    { id: "indoor-temp", label: "Indoor temp", value: "21.4", unit: "°C", sublabel: "Living room" },
    { id: "office-co2", label: "Office CO₂", value: "812", unit: "ppm", sublabel: "Good" },
    { id: "lights-on", label: "Lights on", value: "6", unit: "/ 23", sublabel: "6 rooms active" },
    // TODO(plan-03): replace with EventService.Tail coalescer for events/min
    { id: "events-per-min", label: "Events/min", value: "128", unit: "avg", sublabel: "Last 5 min" },
  ];
}
