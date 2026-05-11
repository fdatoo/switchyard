// TODO(plan-10 wire): replace with AutomationService.List RPC

export type TriggerKind =
  | "sun_event"
  | "time"
  | "entity_state_change"
  | "webhook"
  | "manual";

export interface AutomationTrigger {
  kind: TriggerKind;
  /** Human-readable summary derived from trigger config. */
  summary: string;
}

export interface AutomationSummary {
  id: string;
  displayName: string;
  enabled: boolean;
  trigger: AutomationTrigger;
}

/**
 * useAutomations — returns the list of automations.
 * Mock data; the real impl will call AutomationService.List.
 */
export function useAutomations(): AutomationSummary[] {
  // In a real implementation this would be a query to the gRPC server.
  return [
    {
      id: "sunset-lights",
      displayName: "Sunset Lights",
      enabled: true,
      trigger: { kind: "sun_event", summary: "Sun · sunset −15min" },
    },
    {
      id: "morning-routine",
      displayName: "Morning Routine",
      enabled: true,
      trigger: { kind: "time", summary: "Daily at 6:30 AM" },
    },
    {
      id: "lock-front-door",
      displayName: "Lock Front Door at 11 PM",
      enabled: false,
      trigger: { kind: "time", summary: "Daily at 11:00 PM" },
    },
  ];
}
