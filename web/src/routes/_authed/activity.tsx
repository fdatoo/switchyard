import { useState, useEffect, useCallback } from "react";
import { StoriesTab } from "@/pages/activity/StoriesTab";
import { AllEventsTab } from "@/pages/activity/AllEventsTab";
import { SavedTab } from "@/pages/activity/SavedTab";
import {
  listStories,
  listEvents,
  listSavedQueries,
  saveQuery,
  deleteSavedQuery,
} from "@/data/activity-client";
import type { Story, EventRecord, SavedQuery } from "@/data/activity-client";
import styles from "./activity.module.css";

type ActivityTab = "stories" | "all-events" | "saved";

const TAB_LABELS: Record<ActivityTab, string> = {
  stories: "Stories",
  "all-events": "All Events",
  saved: "Saved",
};

const ALL_TABS: ActivityTab[] = ["stories", "all-events", "saved"];

/**
 * ActivityPage — three-tab Activity view.
 *
 * Tabs:
 *   - Stories: Stories feed (720px column + 380px ContextRail)
 *   - All Events: Faceted event explorer with sparkline, facet rail, table, detail panel
 *   - Saved: Saved queries list with save dialog
 */
export function Activity() {
  const [activeTab, setActiveTab] = useState<ActivityTab>("stories");
  const [stories, setStories] = useState<Story[]>([]);
  const [events, setEvents] = useState<EventRecord[]>([]);
  const [savedQueries, setSavedQueries] = useState<SavedQuery[]>([]);
  const [storiesLoading, setStoriesLoading] = useState(false);
  const [eventsLoading, setEventsLoading] = useState(false);
  const [currentFilter, setCurrentFilter] = useState("");

  // Load stories on mount (and when tab switches to stories).
  useEffect(() => {
    if (activeTab !== "stories") return;
    setStoriesLoading(true);
    listStories()
      .then(setStories)
      .catch(console.error)
      .finally(() => setStoriesLoading(false));
  }, [activeTab]);

  // Load events when switching to all-events tab.
  useEffect(() => {
    if (activeTab !== "all-events") return;
    setEventsLoading(true);
    listEvents()
      .then(setEvents)
      .catch(console.error)
      .finally(() => setEventsLoading(false));
  }, [activeTab]);

  // Load saved queries on mount.
  useEffect(() => {
    listSavedQueries()
      .then(setSavedQueries)
      .catch(console.error);
  }, []);

  const handleSaveQuery = useCallback(
    async (name: string, filter: string, cron: string) => {
      await saveQuery({ name, filter, cron: cron || undefined });
      const updated = await listSavedQueries();
      setSavedQueries(updated);
    },
    [],
  );

  const handleDeleteQuery = useCallback(async (id: string) => {
    await deleteSavedQuery(id);
    const updated = await listSavedQueries();
    setSavedQueries(updated);
  }, []);

  const handleRunQuery = useCallback(
    (query: SavedQuery) => {
      setCurrentFilter(query.filter);
      setActiveTab("all-events");
    },
    [],
  );

  return (
    <div className={styles.page} data-testid="activity-page">
      {/* Tab bar */}
      <nav className={styles.tabBar} role="tablist" aria-label="Activity tabs">
        {ALL_TABS.map((tab) => (
          <button
            key={tab}
            role="tab"
            aria-selected={activeTab === tab}
            aria-controls={`panel-${tab}`}
            className={styles.tab}
            data-active={activeTab === tab ? "true" : undefined}
            onClick={() => setActiveTab(tab)}
          >
            {TAB_LABELS[tab]}
          </button>
        ))}
      </nav>

      {/* Tab panels */}
      <div
        id="panel-stories"
        role="tabpanel"
        aria-labelledby="tab-stories"
        hidden={activeTab !== "stories"}
        className={styles.panel}
      >
        <StoriesTab stories={stories} loading={storiesLoading} />
      </div>

      <div
        id="panel-all-events"
        role="tabpanel"
        aria-labelledby="tab-all-events"
        hidden={activeTab !== "all-events"}
        className={styles.panel}
      >
        <AllEventsTab events={events} loading={eventsLoading} />
      </div>

      <div
        id="panel-saved"
        role="tabpanel"
        aria-labelledby="tab-saved"
        hidden={activeTab !== "saved"}
        className={styles.panel}
      >
        <SavedTab
          queries={savedQueries}
          currentFilter={currentFilter}
          onSaveQuery={handleSaveQuery}
          onDeleteQuery={handleDeleteQuery}
          onRunQuery={handleRunQuery}
        />
      </div>
    </div>
  );
}
