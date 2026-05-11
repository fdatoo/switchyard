// TODO(plan-10 wire): replace with AutomationService.List RPC

export interface AutomationSummary {
  id: string;
  displayName: string;
  enabled: boolean;
}

/**
 * useAutomations — returns the list of automations.
 * Mock data; the real impl will call AutomationService.List.
 */
export function useAutomations(): AutomationSummary[] {
  // In a real implementation this would be a query to the gRPC server.
  return [
    { id: "sunset-lights", displayName: "Sunset Lights", enabled: true },
    { id: "morning-routine", displayName: "Morning Routine", enabled: true },
    { id: "lock-front-door", displayName: "Lock Front Door at 11 PM", enabled: false },
  ];
}
