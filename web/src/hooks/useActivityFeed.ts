/**
 * useActivityFeed — provides activity stories for the mobile activity feed.
 * TODO(plan-3): wire to ActivityService.Stories stream
 *
 * For now, returns hard-coded stories matching the mockup.
 */
import { useHomeActivity } from "@/pages/home/hooks/useHomeActivity";

export interface ActivityStory {
  id: string;
  title: string;
  whyInteresting: string[];
  events: { id: string; summary: string }[];
  actions: string[];
}

export interface ActivityFeed {
  stories: ActivityStory[];
  isLoading: boolean;
}

export function useActivityFeed(): ActivityFeed {
  // TODO(plan-3): wire to ActivityService.Stories stream
  const rawStories = useHomeActivity();

  return {
    stories: rawStories.map((s) => ({
      id: s.id,
      title: s.title,
      whyInteresting: [],
      events: [],
      actions: ["Dismiss", "View details"],
    })),
    isLoading: false,
  };
}
