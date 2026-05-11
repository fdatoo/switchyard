/**
 * recently-used.ts
 * localStorage helpers for recently-used palette commands and CLI preview pref.
 * UI v2 Plan 05.
 */
import { useSyncExternalStore, useCallback } from "react";

// ─── Recently-used records ──────────────────────────────────────────────────

export interface RecentlyUsedRecord {
  verbName: string;
  args: Record<string, string>;
  ranAt: string; // ISO-8601
}

const RECENTLY_USED_KEY = "sy.palette.recentlyUsed";
const MAX_ENTRIES = 50;
const MAX_DISPLAY = 5;
const SEVEN_DAYS_MS = 7 * 24 * 60 * 60 * 1000;

function readRecentlyUsed(): RecentlyUsedRecord[] {
  if (typeof localStorage === "undefined") return [];
  try {
    const raw = localStorage.getItem(RECENTLY_USED_KEY);
    if (!raw) return [];
    return JSON.parse(raw) as RecentlyUsedRecord[];
  } catch {
    return [];
  }
}

function writeRecentlyUsed(records: RecentlyUsedRecord[]): void {
  if (typeof localStorage === "undefined") return;
  localStorage.setItem(RECENTLY_USED_KEY, JSON.stringify(records));
}

/**
 * Record a command execution. Deduplicates by exact verb+args match (most recent wins).
 * Caps the list at MAX_ENTRIES.
 */
export function recordRecentlyUsed(
  verbName: string,
  args: Record<string, string>,
): void {
  const existing = readRecentlyUsed();
  const argsKey = JSON.stringify(args);
  // Remove any existing entry with the same verb+args.
  const deduped = existing.filter(
    (r) => !(r.verbName === verbName && JSON.stringify(r.args) === argsKey),
  );
  const newEntry: RecentlyUsedRecord = {
    verbName,
    args,
    ranAt: new Date().toISOString(),
  };
  const updated = [newEntry, ...deduped].slice(0, MAX_ENTRIES);
  writeRecentlyUsed(updated);
  // Notify listeners.
  recentlyUsedListeners.forEach((l) => l());
}

// ─── useSyncExternalStore subscription ──────────────────────────────────────

const recentlyUsedListeners = new Set<() => void>();

function subscribeRecentlyUsed(onChange: () => void): () => void {
  recentlyUsedListeners.add(onChange);
  return () => recentlyUsedListeners.delete(onChange);
}

function getRecentlyUsedSnapshot(): RecentlyUsedRecord[] {
  return readRecentlyUsed();
}

/**
 * Hook: returns the last 7 days of recently-used records (up to MAX_DISPLAY for the UI;
 * callers can slice further). Reactive to writes via useSyncExternalStore.
 */
export function useRecentlyUsed(limit = MAX_DISPLAY): RecentlyUsedRecord[] {
  const records = useSyncExternalStore(
    subscribeRecentlyUsed,
    getRecentlyUsedSnapshot,
    () => [] as RecentlyUsedRecord[],
  );
  const cutoff = Date.now() - SEVEN_DAYS_MS;
  return records
    .filter((r) => new Date(r.ranAt).getTime() >= cutoff)
    .slice(0, limit);
}

// ─── CLI preview preference ──────────────────────────────────────────────────

const CLI_PREVIEW_KEY = "sy.palette.cliPreview";

export function getPaletteCliPreview(): boolean {
  if (typeof localStorage === "undefined") return false;
  return localStorage.getItem(CLI_PREVIEW_KEY) === "on";
}

export function setPaletteCliPreview(on: boolean): void {
  if (typeof localStorage === "undefined") return;
  localStorage.setItem(CLI_PREVIEW_KEY, on ? "on" : "off");
  cliPreviewListeners.forEach((l) => l());
}

const cliPreviewListeners = new Set<() => void>();

function subscribeCLIPreview(onChange: () => void): () => void {
  cliPreviewListeners.add(onChange);
  // Also listen for cross-tab storage events.
  const handler = (e: StorageEvent) => {
    if (e.key === CLI_PREVIEW_KEY) onChange();
  };
  if (typeof window !== "undefined") {
    window.addEventListener("storage", handler);
  }
  return () => {
    cliPreviewListeners.delete(onChange);
    if (typeof window !== "undefined") {
      window.removeEventListener("storage", handler);
    }
  };
}

function getCLIPreviewSnapshot(): boolean {
  return getPaletteCliPreview();
}

/**
 * Hook: returns [cliPreviewOn, setCLIPreview]. Reactive via useSyncExternalStore.
 * Subscribes to both in-tab writes and cross-tab storage events.
 */
export function usePaletteCliPreview(): [boolean, (on: boolean) => void] {
  const value = useSyncExternalStore(
    subscribeCLIPreview,
    getCLIPreviewSnapshot,
    () => false,
  );
  const set = useCallback((on: boolean) => setPaletteCliPreview(on), []);
  return [value, set];
}

