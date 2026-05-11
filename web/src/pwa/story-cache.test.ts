import "fake-indexeddb/auto";
import { expect, test, beforeEach } from "vitest";
import { cacheStories, loadCachedStories } from "./story-cache";

// Reset indexedDB between tests by re-importing fake-indexeddb/auto each time.
// fake-indexeddb/auto patches globalThis.indexedDB with a fresh instance.
beforeEach(async () => {
  // Close any open connections and reset the DB
  const { IDBFactory } = await import("fake-indexeddb");
  Object.defineProperty(globalThis, "indexedDB", {
    value: new IDBFactory(),
    writable: true,
    configurable: true,
  });
});

// idb uses IndexedDB — polyfilled by fake-indexeddb
test("round-trips up to 50 stories", async () => {
  const stories = Array.from({ length: 60 }, (_, i) => ({
    id: `story-${i}`,
    title: `Story ${i}`,
  }));
  await cacheStories(stories);
  const loaded = await loadCachedStories();
  // only the most recent 50 are kept
  expect(loaded.length).toBe(50);
  expect(loaded[0].id).toBe("story-59");
});
