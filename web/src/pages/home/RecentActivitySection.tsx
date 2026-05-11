import { useHomeActivity, type ActivityStory } from "./hooks/useHomeActivity";

interface RecentActivitySectionProps {
  /** Allow passing mock stories for testing */
  stories?: ActivityStory[];
}

/**
 * RecentActivitySection — renders the top 3 recent activity stories.
 * Each row has a colored left-edge indicator, story title, and relative timestamp.
 */
export function RecentActivitySection({ stories }: RecentActivitySectionProps) {
  const defaultStories = useHomeActivity();
  const activityStories = stories ?? defaultStories;

  return (
    <section>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          marginBottom: "var(--sy-space-3)",
        }}
      >
        <h2
          style={{
            margin: 0,
            fontSize: "0.8125rem",
            fontWeight: 600,
            letterSpacing: "0.06em",
            textTransform: "uppercase",
            color: "var(--sy-color-fg-4)",
          }}
        >
          Recent Activity
        </h2>
        <a
          href="/activity"
          style={{
            fontSize: "0.8125rem",
            color: "var(--sy-color-accent)",
            textDecoration: "none",
            fontWeight: 500,
          }}
        >
          All activity →
        </a>
      </div>
      <div
        style={{
          background: "var(--sy-color-surface-1)",
          border: "1px solid var(--sy-color-line)",
          borderRadius: "var(--sy-radius-lg)",
          overflow: "hidden",
        }}
      >
        {activityStories.map((story, idx) => (
          <div
            key={story.id}
            data-testid="activity-row"
            style={{
              display: "flex",
              alignItems: "center",
              gap: "var(--sy-space-3)",
              padding: "var(--sy-space-3) var(--sy-space-4)",
              borderTop: idx === 0 ? "none" : "1px solid var(--sy-color-line-soft)",
            }}
          >
            {/* Colored left-edge indicator using severity token */}
            <span
              style={{
                width: "3px",
                height: "2rem",
                borderRadius: "var(--sy-radius-pill)",
                background: story.severityColor,
                flexShrink: 0,
              }}
            />
            <div
              style={{
                flex: 1,
                minWidth: 0,
              }}
            >
              <div
                style={{
                  fontSize: "0.875rem",
                  fontWeight: 500,
                  color: "var(--sy-color-fg)",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {story.title}
              </div>
              <div
                style={{
                  fontSize: "0.75rem",
                  color: "var(--sy-color-fg-4)",
                  marginTop: "1px",
                }}
              >
                {story.kindPill}
              </div>
            </div>
            <span
              style={{
                fontSize: "0.75rem",
                color: "var(--sy-color-fg-4)",
                flexShrink: 0,
              }}
            >
              {story.relativeTime}
            </span>
          </div>
        ))}
      </div>
    </section>
  );
}
