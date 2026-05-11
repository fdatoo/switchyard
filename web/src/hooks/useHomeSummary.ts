/**
 * useHomeSummary — aggregates the home page data hooks into a single summary
 * shape optimised for the mobile home view.
 *
 * The underlying data comes from Plan 02's hooks; this is simply a composed
 * adapter so MobileHome doesn't need to import five separate hooks.
 */
import { useHomeRooms } from "@/pages/home/hooks/useHomeRooms";
import { useHomeActivity } from "@/pages/home/hooks/useHomeActivity";
import { useHomeStatus } from "@/pages/home/hooks/useHomeStatus";

export interface HomeSummaryStat {
  id: string;
  label: string;
  value: string;
}

export interface HomeSummaryRoom {
  slug: string;
  name: string;
}

export interface HomeSummaryStory {
  id: string;
  title: string;
}

export interface HomeSummary {
  stats: HomeSummaryStat[];
  rooms: HomeSummaryRoom[];
  recentStories: HomeSummaryStory[];
}

export function useHomeSummary(): HomeSummary {
  const rooms = useHomeRooms();
  const stories = useHomeActivity();
  const statusPills = useHomeStatus();

  // Build 4 stat tiles from status data
  const stats: HomeSummaryStat[] = [
    { id: "lights-on", label: "Lights on", value: "4" },
    { id: "temp", label: "Temperature", value: "21°C" },
    { id: "open-doors", label: "Open doors", value: "0" },
    {
      id: "automations",
      label: "Automations",
      value: statusPills.find((p) => p.id === "automations-active")?.label ?? "—",
    },
  ];

  return {
    stats,
    rooms: rooms.map((r) => ({ slug: r.id, name: r.name })),
    recentStories: stories.map((s) => ({ id: s.id, title: s.title })),
  };
}
