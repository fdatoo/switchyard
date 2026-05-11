import { useEffect, useSyncExternalStore } from "react";
import {
  getDaemonConnectionSnapshot,
  markDaemonReachable,
  markDaemonUnreachable,
  subscribeDaemonConnection,
} from "@/data/daemon-connection";

type ReconnectingBannerProps = {
  healthPath?: string;
  retryIntervalMs?: number;
};

export function ReconnectingBanner({
  healthPath = "/health",
  retryIntervalMs = 1000,
}: ReconnectingBannerProps) {
  const connection = useSyncExternalStore(
    subscribeDaemonConnection,
    getDaemonConnectionSnapshot,
    getDaemonConnectionSnapshot,
  );

  useEffect(() => {
    if (connection.status !== "reconnecting") return;

    let cancelled = false;
    const checkHealth = async () => {
      try {
        const response = await fetch(healthPath, {
          cache: "no-store",
          credentials: "same-origin",
        });
        if (cancelled) return;
        if (response.ok) {
          markDaemonReachable();
        } else {
          markDaemonUnreachable(new Error(`health check failed: ${response.status}`));
        }
      } catch (error) {
        if (!cancelled) {
          markDaemonUnreachable(error);
        }
      }
    };

    void checkHealth();
    const timer = window.setInterval(checkHealth, retryIntervalMs);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [connection.status, healthPath, retryIntervalMs]);

  if (connection.status !== "reconnecting") return null;

  return (
    <div className="reconnecting-banner" role="status" aria-live="polite">
      <strong>Reconnecting to gohome</strong>
      <span>Retrying health check...</span>
    </div>
  );
}
