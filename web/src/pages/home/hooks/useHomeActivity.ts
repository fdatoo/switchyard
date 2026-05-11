// TODO(plan-03): replace with EventService.Tail stories coalescer

export interface ActivityStory {
  id: string;
  title: string;
  kindPill: string;
  relativeTime: string;
  /** CSS token reference e.g. "var(--sy-color-bad)" */
  severityColor: string;
}

/**
 * useHomeActivity — returns top 3 recent activity stories.
 * Mock data; Plan 03 will wire to EventService.Tail stories coalescer.
 */
export function useHomeActivity(): ActivityStory[] {
  return [
    {
      id: "act-1",
      title: "Motion detected in Hallway",
      kindPill: "Alert",
      relativeTime: "2 min ago",
      severityColor: "var(--sy-color-warn)",
    },
    {
      id: "act-2",
      title: "Front door locked automatically",
      kindPill: "Automation",
      relativeTime: "18 min ago",
      severityColor: "var(--sy-color-good)",
    },
    {
      id: "act-3",
      title: "Office CO₂ above threshold",
      kindPill: "Alert",
      relativeTime: "1 hr ago",
      severityColor: "var(--sy-color-bad)",
    },
  ];
}
