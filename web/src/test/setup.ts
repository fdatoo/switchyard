import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

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
