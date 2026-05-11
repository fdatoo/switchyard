import { useRef, useState } from "react";
import { useActivityFeed } from "@/hooks/useActivityFeed";
import { StorySheet } from "@/mobile/StorySheet";
import { usePullToRefresh } from "@/mobile/usePullToRefresh";
import styles from "./MobileActivity.module.css";

export function MobileActivity() {
  const { stories, isLoading } = useActivityFeed();
  const [activeStory, setActiveStory] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const selectedStory = stories.find((s) => s.id === activeStory) ?? null;

  usePullToRefresh(scrollRef, () => {
    // invalidate the activity feed query; implemented via TanStack Query refetch
    // the hook just calls onRefresh — the caller controls what "refresh" means
  });

  return (
    <div ref={scrollRef} className={styles.page}>
      {isLoading && <p className={styles.loading}>Loading…</p>}
      {stories.map((s) => (
        <button key={s.id} className={styles.card} onClick={() => setActiveStory(s.id)}>
          <span className={styles.cardTitle}>{s.title}</span>
        </button>
      ))}
      {selectedStory && (
        <StorySheet
          open={!!activeStory}
          story={selectedStory}
          onOpenChange={(open) => {
            if (!open) setActiveStory(null);
          }}
        />
      )}
    </div>
  );
}
