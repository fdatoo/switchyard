import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach, beforeEach } from "vitest";

// In-memory localStorage shim for tests that use it.
// jsdom provides window.localStorage but it may be limited in some envs.
class MemoryLocalStorage {
  private store: Map<string, string> = new Map();
  getItem(key: string): string | null {
    return this.store.get(key) ?? null;
  }
  setItem(key: string, value: string): void {
    this.store.set(key, value);
  }
  removeItem(key: string): void {
    this.store.delete(key);
  }
  clear(): void {
    this.store.clear();
  }
  get length(): number {
    return this.store.size;
  }
  key(index: number): string | null {
    return [...this.store.keys()][index] ?? null;
  }
}

const memLS = new MemoryLocalStorage();
Object.defineProperty(window, "localStorage", {
  configurable: true,
  get: () => memLS,
});

// Reset localStorage between tests.
beforeEach(() => {
  memLS.clear();
});

// Automatically cleanup after each test
afterEach(() => {
  cleanup();
});

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: (query: string): MediaQueryList => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  }),
});

class MemoryIDBRequest extends EventTarget {
  result: unknown = undefined;
  error: DOMException | null = null;
  onsuccess: ((event: Event) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  succeed(result: unknown) {
    this.result = result;
    const event = new Event("success");
    this.onsuccess?.(event);
    this.dispatchEvent(event);
  }
}

const indexedDBShim = {
  open: () => {
    const request = new MemoryIDBRequest();
    queueMicrotask(() => request.succeed({ close: () => undefined }));
    return request;
  },
  deleteDatabase: () => {
    const request = new MemoryIDBRequest();
    queueMicrotask(() => request.succeed(undefined));
    return request;
  },
};

Object.defineProperty(window, "indexedDB", {
  configurable: true,
  value: indexedDBShim,
});
