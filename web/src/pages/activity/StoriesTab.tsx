import { useState } from "react";
import type { Story, StoriesFilter } from "../../gen/activity/v1/activity_pb";
import { StoryCard } from "./StoryCard";
import { ContextRail } from "./ContextRail";
import styles from "./StoriesTab.module.css";

export interface StoriesTabProps {
  /** Stories to display. Caller provides (loaded via useQuery or mock). */
  stories: Story[];
  filter?: StoriesFilter;
  loading?: boolean;
}

/**
 * StoriesTab — the main feed layout for the Activity page.
 *
 * Layout: two-column grid.
 *   - Left/center: max-width 720px feed of StoryCard components.
 *   - Right: 380px ContextRail, always visible on wide screens (≥ 1100px).
 *
 * Click a story card → ContextRail populates. No inline expansion.
 */
export function StoriesTab({ stories, loading = false }: StoriesTabProps) {
  const [selectedStory, setSelectedStory] = useState<Story | null>(null);

  return (
    <div className={styles.layout} data-testid="stories-tab">
      {/* Feed column */}
      <div className={styles.feed}>
        {loading && (
          <div className={styles.loading} role="status" aria-live="polite">
            Loading stories…
          </div>
        )}
        {!loading && stories.length === 0 && (
          <div className={styles.empty}>No stories found.</div>
        )}
        {stories.map((story) => (
          <StoryCard
            key={story.id}
            story={story}
            selected={selectedStory?.id === story.id}
            onSelect={setSelectedStory}
          />
        ))}
      </div>

      {/* Context rail */}
      <ContextRail story={selectedStory} />
    </div>
  );
}
