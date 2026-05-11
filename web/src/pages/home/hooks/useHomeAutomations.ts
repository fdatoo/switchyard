// TODO(plan-10): replace with AutomationService.List

export interface AutomationItem {
  id: string;
  name: string;
  /** Human-readable time label, e.g. "in 47 min" or "10:30 PM" */
  timeLabel: string;
}

/**
 * useHomeAutomations — returns active automations (recently-fired or due within 1h).
 * Mock data; Plan 10 will wire to AutomationService.List.
 */
export function useHomeAutomations(): AutomationItem[] {
  return [
    { id: "auto-1", name: "Lock front door at 11 PM", timeLabel: "in 47 min" },
    { id: "auto-2", name: "Morning routine", timeLabel: "6:30 AM" },
    { id: "auto-3", name: "Arrive home scene", timeLabel: "Ran 10 min ago" },
  ];
}
