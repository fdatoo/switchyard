import { useSyncExternalStore } from "react";
import { useMultiplexer } from "./multiplexer-context";

export function useEntityState(id: string) {
  const mux = useMultiplexer();
  return useSyncExternalStore(
    cb => mux.cache.subscribe(id, cb),
    () => mux.cache.get(id),
  );
}

export function usePending(id: string) {
  const mux = useMultiplexer();
  return useSyncExternalStore(
    cb => mux.tracker.subscribe(id, cb),
    () => mux.tracker.current(id),
  );
}

export function useCallCapability() {
  const mux = useMultiplexer();
  return mux.callCapability;
}
