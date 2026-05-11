/**
 * AlertPill — ambient alert component.
 *
 * Subscribes to ActivityService.Stories (filtered to failure/causation tags).
 * When an event arrives at or above the display's alert_threshold, surfaces a
 * glassmorphic pill at the top of the display with the alert message.
 *
 * Also exposes an AlertContext so RoomTile components can dim themselves.
 */

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { usePrimitive } from "@/theme/primitives-provider";
import type { PillProps } from "@/theme/primitives/pill";

// ---------------------------------------------------------------------------
// Alert context — shared state between AlertPill and RoomTile
// ---------------------------------------------------------------------------

export type AlertThreshold = "NONE" | "LOW" | "MEDIUM" | "HIGH";

export interface AlertState {
  active: boolean;
  message: string;
  affectedEntityIds: string[];
}

interface AlertContextValue {
  alertState: AlertState;
  alertThreshold: AlertThreshold;
}

const AlertCtx = createContext<AlertContextValue>({
  alertState: { active: false, message: "", affectedEntityIds: [] },
  alertThreshold: "NONE",
});

// ---------------------------------------------------------------------------
// Severity ordering
// ---------------------------------------------------------------------------

const SEVERITY_ORDER: Record<AlertThreshold, number> = {
  NONE: 0,
  LOW: 1,
  MEDIUM: 2,
  HIGH: 3,
};

function severityOf(category: string): AlertThreshold {
  switch (category) {
    case "failure":   return "HIGH";
    case "causation": return "MEDIUM";
    case "anomaly":   return "LOW";
    default:          return "NONE";
  }
}

// ---------------------------------------------------------------------------
// ActivityService stream client (minimal subset)
// ---------------------------------------------------------------------------

interface InterestingnessTag {
  category: string;
  name: string;
  explanation: string;
}

interface StoryEvent {
  id: string;
  title: string;
  entityIds?: string[];
  tags: InterestingnessTag[];
}

type CloseStream = () => void;

function subscribeToInterestingStories(
  onStory: (story: StoryEvent) => void,
): CloseStream {
  // Use EventSource over the Connect HTTP streaming endpoint.
  // The ActivityService.Stories RPC is a server-streaming call; the Connect
  // protocol encodes each message as a length-prefixed envelope over HTTP/1.1.
  // For simplicity we poll via fetch abort + streaming body reader.
  const controller = new AbortController();
  let active = true;

  async function stream() {
    try {
      const res = await fetch("/switchyard.activity.v1.ActivityService/Stories", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Connect-Protocol-Version": "1",
        },
        body: JSON.stringify({ interesting_only: true }),
        credentials: "include",
        signal: controller.signal,
      });
      if (!res.ok || !res.body) return;
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";
      while (active) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        // Parse newline-delimited JSON from Connect envelope
        const lines = buf.split("\n");
        buf = lines.pop() ?? "";
        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed) continue;
          try {
            const msg = JSON.parse(trimmed) as { stories?: StoryEvent[] };
            if (msg.stories) {
              for (const story of msg.stories) {
                onStory(story);
              }
            }
          } catch { /* skip malformed frames */ }
        }
      }
    } catch { /* network error — silent */ }
  }

  void stream();
  return () => {
    active = false;
    controller.abort();
  };
}

// ---------------------------------------------------------------------------
// AlertContext provider
// ---------------------------------------------------------------------------

interface AlertContextProps {
  children: ReactNode;
  alertThreshold: AlertThreshold;
}

export function AlertContext({ children, alertThreshold }: AlertContextProps) {
  const [alertState, setAlertState] = useState<AlertState>({
    active: false,
    message: "",
    affectedEntityIds: [],
  });

  useEffect(() => {
    const close = subscribeToInterestingStories((story) => {
      // Find the highest-severity tag in the story.
      let maxSeverity: AlertThreshold = "NONE";
      for (const tag of story.tags) {
        const sev = severityOf(tag.category);
        if (SEVERITY_ORDER[sev] > SEVERITY_ORDER[maxSeverity]) {
          maxSeverity = sev;
        }
      }

      if (SEVERITY_ORDER[maxSeverity] >= SEVERITY_ORDER[alertThreshold] &&
          alertThreshold !== "NONE") {
        setAlertState({
          active: true,
          message: story.title,
          affectedEntityIds: story.entityIds ?? [],
        });
      }
    });
    return close;
  }, [alertThreshold]);

  return (
    <AlertCtx.Provider value={{ alertState, alertThreshold }}>
      {children}
    </AlertCtx.Provider>
  );
}

// ---------------------------------------------------------------------------
// useAlertState hook
// ---------------------------------------------------------------------------

// eslint-disable-next-line react-refresh/only-export-components
export function useAlertState(): AlertContextValue {
  return useContext(AlertCtx);
}

// ---------------------------------------------------------------------------
// AlertPill component
// ---------------------------------------------------------------------------

interface AlertPillProps {
  alertThreshold: AlertThreshold;
}

export function AlertPill({ alertThreshold }: AlertPillProps) {
  const { alertState } = useContext(AlertCtx);
  const AmbientPillComponent = usePrimitive("Pill");

  if (!alertState.active || alertThreshold === "NONE") {
    return null;
  }

  return (
    <div
      data-testid="alert-pill"
      style={{
        position: "fixed",
        top: "var(--sy-space-3, 0.75rem)",
        left: "50%",
        transform: "translateX(-50%)",
        zIndex: 1000,
        maxWidth: "90vw",
      }}
    >
      <AmbientPillComponent
        variant="bad"
        style={
          {
            padding: "var(--sy-space-2) var(--sy-space-4)",
            fontSize: "0.875rem",
            gap: "var(--sy-space-2)",
          } as PillProps["style"]
        }
      >
        ⚠ {alertState.message}
      </AmbientPillComponent>
    </div>
  );
}
