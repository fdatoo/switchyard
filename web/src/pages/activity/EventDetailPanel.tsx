import type { EventRecord } from "../../gen/activity/v1/activity_pb";
import styles from "./EventDetailPanel.module.css";

function navigateToTimeMachine(eventId: string): void {
  window.location.href = `/_authed/time-machine/${encodeURIComponent(eventId)}`;
}

export interface EventDetailPanelProps {
  event: EventRecord | null;
  onClose: () => void;
}

/**
 * EventDetailPanel — 420px slide-in right panel.
 *
 * Sections:
 *   - Identity (event_id, command_id, causation_id, correlation_id, OTel)
 *   - Source
 *   - Payload JSON (collapsible)
 *   - Causation chain
 *   - Actions
 */
export function EventDetailPanel({ event, onClose }: EventDetailPanelProps) {
  if (!event) {
    return null;
  }

  return (
    <aside
      className={styles.panel}
      role="complementary"
      aria-label="Event detail"
      data-testid="event-detail-panel"
    >
      <div className={styles.panelHeader}>
        <h2 className={styles.panelTitle}>Event Detail</h2>
        <button
          className={styles.closeBtn}
          onClick={onClose}
          aria-label="Close event detail"
        >
          ×
        </button>
      </div>

      <div className={styles.sections}>
        {/* Identity */}
        <section className={styles.section}>
          <h3 className={styles.sectionHeading}>Identity</h3>
          <dl className={styles.dl}>
            <dt>Event ID</dt>
            <dd data-testid="event-id">
              <code>{event.eventId || "—"}</code>
            </dd>
            <dt>Correlation ID</dt>
            <dd>
              <code>{event.correlationId || "—"}</code>
            </dd>
            <dt>Causation ID</dt>
            <dd>
              <code>{event.causationId || "—"}</code>
            </dd>
            <dt>OTel Trace</dt>
            <dd>
              <code>{event.otelTraceId || "—"}</code>
            </dd>
            <dt>OTel Span</dt>
            <dd>
              <code>{event.otelSpanId || "—"}</code>
            </dd>
          </dl>
        </section>

        {/* Source */}
        <section className={styles.section}>
          <h3 className={styles.sectionHeading}>Source</h3>
          <dl className={styles.dl}>
            <dt>Source</dt>
            <dd>{event.source || "—"}</dd>
            <dt>Entity</dt>
            <dd>{event.entity || "—"}</dd>
            <dt>Kind</dt>
            <dd>{event.kind || "—"}</dd>
          </dl>
        </section>

        {/* Payload JSON */}
        {event.payloadJson && (
          <section className={styles.section}>
            <h3 className={styles.sectionHeading}>Payload</h3>
            <details>
              <summary className={styles.payloadSummary}>View raw JSON</summary>
              <pre className={styles.payloadPre}>{event.payloadJson}</pre>
            </details>
          </section>
        )}

        {/* Actions */}
        <div className={styles.actions}>
          <button className={styles.actionBtn} disabled aria-label="Repeat command (coming soon)">
            Repeat command
          </button>
          {event.sequence !== "0" && event.sequence !== undefined && (
            <button
              className={styles.actionBtn}
              onClick={() => navigateToTimeMachine(event.eventId)}
              aria-label="Replay in Time-machine"
              data-testid="replay-in-time-machine-btn"
            >
              Replay in Time-machine
            </button>
          )}
          <button
            className={styles.actionBtn}
            onClick={() => {
              void navigator.clipboard.writeText(
                `curl -X POST http://localhost:7070/api/events/${event.eventId}`,
              );
            }}
            aria-label="Copy as cURL"
          >
            Copy as cURL
          </button>
        </div>
      </div>
    </aside>
  );
}
