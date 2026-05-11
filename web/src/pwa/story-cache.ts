/**
 * story-cache.ts — IndexedDB read/write for the last-50 activity stories.
 *
 * Uses the browser-native indexedDB API directly (no wrappers).
 * On cold load the cached stories are read and injected into the activity
 * feed before the multiplexer reconnects.
 */

const DB_NAME = "sy-pwa";
const STORE = "stories";
const VERSION = 1;
const MAX_STORIES = 50;

export interface CachedStory {
  id: string;
  title: string;
}

// Promisify IDBOpenDBRequest into a resolved IDBDatabase
function openDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, VERSION);
    req.onupgradeneeded = () => {
      if (!req.result.objectStoreNames.contains(STORE)) {
        req.result.createObjectStore(STORE, { keyPath: "id" });
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

/** Persist up to MAX_STORIES (most-recent) stories to IndexedDB. */
export async function cacheStories(stories: CachedStory[]): Promise<void> {
  const db = await openDB();
  const recent = stories.slice(-MAX_STORIES);

  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE, "readwrite");
    const store = tx.objectStore(STORE);
    const clearReq = store.clear();
    clearReq.onsuccess = () => {
      for (const s of recent) {
        store.put(s);
      }
    };
    tx.oncomplete = () => { db.close(); resolve(); };
    tx.onerror = () => { db.close(); reject(tx.error); };
  });
}

/** Load all cached stories from IndexedDB, sorted most-recent first. */
export async function loadCachedStories(): Promise<CachedStory[]> {
  const db = await openDB();

  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE, "readonly");
    const req = tx.objectStore(STORE).getAll();
    req.onsuccess = () => {
      db.close();
      const all = req.result as CachedStory[];
      // Sort descending by id (ids are lexicographically ordered by insertion time)
      resolve(all.sort((a, b) => b.id.localeCompare(a.id)));
    };
    req.onerror = () => { db.close(); reject(req.error); };
  });
}
