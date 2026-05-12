/**
 * useDaemonStatus — polls `/healthz` and exposes a reactive status flag.
 *
 * v1: a single periodic fetch every `intervalMs` (default 10s) sets the
 * status to `ok` on 2xx and `down` on anything else. The `checking`
 * state covers the initial moment before the first response. A future
 * version can subscribe to a daemon-level SSE/WS for instant updates;
 * the consumer-facing shape (`status` ref + `refresh()` action) is
 * stable across those implementations.
 *
 * Cleanup: the interval is cleared on `onUnmounted` so leaving the
 * authed area doesn't keep polling, and the in-flight fetch is aborted
 * on each new tick to prevent stale responses arriving after a newer
 * tick has resolved.
 */

import { onUnmounted, ref } from "vue";

export type DaemonStatus = "ok" | "reconnecting" | "down" | "checking";

interface Options {
  /** Poll interval in ms. Default 10s. */
  intervalMs?: number;
  /** Endpoint to poll. Defaults to the daemon's healthz. */
  url?: string;
}

export function useDaemonStatus(opts: Options = {}) {
  const intervalMs = opts.intervalMs ?? 10_000;
  const url = opts.url ?? "/healthz";

  const status = ref<DaemonStatus>("checking");
  let inflight: AbortController | null = null;
  let timer: ReturnType<typeof setInterval> | null = null;

  async function refresh(): Promise<void> {
    inflight?.abort();
    inflight = new AbortController();
    try {
      const res = await fetch(url, { credentials: "include", signal: inflight.signal });
      status.value = res.ok ? "ok" : "down";
    } catch (err) {
      if ((err as Error).name === "AbortError") return;
      status.value = "down";
    }
  }

  refresh();
  timer = setInterval(refresh, intervalMs);

  onUnmounted(() => {
    if (timer) clearInterval(timer);
    inflight?.abort();
  });

  return { status, refresh };
}
