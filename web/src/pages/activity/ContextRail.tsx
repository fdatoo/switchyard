import type { Story } from "../../gen/activity/v1/activity_pb";
import { InterestingnessBadge } from "./InterestingnessBadge";
import styles from "./ContextRail.module.css";

export interface ContextRailProps {
  story: Story | null;
  onCopyEventIds?: (ids: string[]) => void;
}

/**
 * ContextRail — 380px right panel showing story detail.
 *
 * Sections:
 *   - Story title + ISO timestamp
 *   - "Why interesting" — one why-item card per tag (badge + name + explanation)
 *   - Inner events timeline
 *   - Actions row
 */
export function ContextRail({ story, onCopyEventIds }: ContextRailProps) {
  if (!story) {
    return (
      <aside className={styles.rail} aria-label="Story detail" data-testid="context-rail">
        <div className={styles.empty}>
          <p>Select a story to see details</p>
        </div>
      </aside>
    );
  }

  const handleCopyIds = () => {
    const ids = story.innerEventIds ?? [];
    const text = ids.join("\n");
    void navigator.clipboard.writeText(text);
    onCopyEventIds?.(ids);
  };

  return (
    <aside
      className={styles.rail}
      aria-label="Story detail"
      data-testid="context-rail"
    >
      <div className={styles.header}>
        <h2 className={styles.title} data-testid="context-rail-title">
          {story.title}
        </h2>
        {story.occurredAt && (
          <time className={styles.timestamp} dateTime={story.occurredAt}>
            {new Date(story.occurredAt).toISOString()}
          </time>
        )}
      </div>

      {story.tags.length > 0 && (
        <section className={styles.section} aria-labelledby="why-heading">
          <h3 className={styles.sectionHeading} id="why-heading">
            Why interesting
          </h3>
          <div className={styles.whyItems}>
            {story.tags.map((tag) => (
              <div
                key={`${tag.category}/${tag.name}`}
                className={styles.whyItem}
                data-testid="why-item"
              >
                <InterestingnessBadge
                  category={tag.category}
                  name={tag.name}
                />
                <p className={styles.explanation}>{tag.explanation}</p>
              </div>
            ))}
          </div>
        </section>
      )}

      <section className={styles.section} aria-labelledby="timeline-heading">
        <h3 className={styles.sectionHeading} id="timeline-heading">
          Events
        </h3>
        <ol className={styles.timeline}>
          {(story.innerEventIds ?? []).map((id, idx) => (
            <li
              key={id}
              className={styles.timelineRow}
              data-testid="timeline-row"
            >
              <span className={styles.timelineNum}>{idx + 1}</span>
              <code className={styles.eventId}>{id}</code>
            </li>
          ))}
        </ol>
      </section>

      <div className={styles.actions}>
        {/* "Repeat command" — Plan 10 stub */}
        <button className={styles.actionBtn} disabled aria-label="Repeat command (coming soon)">
          Repeat command
        </button>
        {/* "Open in Time-machine" — Plan 4 stub */}
        <button className={styles.actionBtn} disabled aria-label="Open in Time-machine (coming soon)">
          Time-machine
        </button>
        <button className={styles.actionBtn} onClick={handleCopyIds} aria-label="Copy event IDs">
          Copy event IDs
        </button>
      </div>
    </aside>
  );
}
