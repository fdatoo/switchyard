import type { Story } from "../../gen/activity/v1/activity_pb";
import { InterestingnessBadge } from "./InterestingnessBadge";
import styles from "./StoryCard.module.css";

export interface StoryCardProps {
  story: Story;
  selected?: boolean;
  onSelect: (story: Story) => void;
}

/** Formats an ISO timestamp as relative time. */
function relativeTime(isoTs: string | null | undefined): string {
  if (!isoTs) return "";
  const date = new Date(isoTs);
  if (isNaN(date.getTime())) return "";
  const diffMs = Date.now() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  if (diffSec < 60) return `${diffSec}s ago`;
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  return `${Math.floor(diffHr / 24)}d ago`;
}

/**
 * StoryCard renders a single story in the Stories feed.
 * Clicking the card calls onSelect(story) so the ContextRail can populate.
 */
export function StoryCard({ story, selected = false, onSelect }: StoryCardProps) {
  return (
    <article
      className={styles.card}
      data-selected={selected ? "true" : undefined}
      onClick={() => onSelect(story)}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") onSelect(story);
      }}
      aria-pressed={selected}
    >
      <div className={styles.header}>
        <span className={styles.title}>{story.title}</span>
        <time className={styles.time} dateTime={story.occurredAt ?? undefined}>
          {relativeTime(story.occurredAt)}
        </time>
      </div>

      <div className={styles.chips}>
        {story.source && (
          <span className={styles.sourceChip}>{story.source}</span>
        )}
        {story.entityIds.slice(0, 3).map((eid) => (
          <span key={eid} className={styles.entityChip}>{eid}</span>
        ))}
        {story.entityIds.length > 3 && (
          <span className={styles.entityChip}>+{story.entityIds.length - 3} more</span>
        )}
      </div>

      {story.tags.length > 0 && (
        <div className={styles.badges}>
          {story.tags.map((tag) => (
            <InterestingnessBadge
              key={`${tag.category}/${tag.name}`}
              category={tag.category}
              name={tag.name}
            />
          ))}
        </div>
      )}
    </article>
  );
}
