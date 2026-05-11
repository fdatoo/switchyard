// TODO(plan-03): replace with real EntityService + interestingness data

export type StatusSeverity = "good" | "warn" | "bad" | "neutral";

export interface StatusPillItem {
  id: string;
  label: string;
  severity: StatusSeverity;
}

/**
 * useHomeStatus — returns status pill data for the status row.
 * Mock data; Plan 03 will wire this to live EntityService + interestingness.
 */
export function useHomeStatus(): StatusPillItem[] {
  return [
    { id: "entities-online", label: "87 of 94 online", severity: "good" },
    { id: "automations-active", label: "12 automations active", severity: "neutral" },
    { id: "config-applied", label: "Config applied 2h ago", severity: "neutral" },
  ];
}
