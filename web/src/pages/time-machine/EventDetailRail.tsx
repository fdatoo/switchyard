import styles from "./EventDetailRail.module.css";
import type { ChainEvent, LoadAtSeqResponse } from "../../data/replay-client";

export interface EventDetailRailProps {
  step: ChainEvent | null;
  state: LoadAtSeqResponse | null;
}

/** Highlight JSON with CSS classes by simple regex replacement. */
function highlightJSON(json: string): string {
  return json
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"([^"]+)":/g, '<span class="json-key">"$1"</span>:')
    .replace(/: "([^"]*)"/g, ': <span class="json-str">"$1"</span>')
    .replace(/: (-?\d+\.?\d*)/g, ': <span class="json-num">$1</span>');
}

function formatTimestamp(iso: string | undefined): string {
  if (!iso) return "—";
  try {
    return new Date(iso).toISOString().replace("T", " ").replace("Z", " UTC");
  } catch {
    return iso;
  }
}

/**
 * EventDetailRail — right panel with event identity, diff, and payload.
 */
export function EventDetailRail({ step, state }: EventDetailRailProps) {
  if (!step) {
    return (
      <aside className={styles.rail} aria-label="Event detail" data-testid="event-detail-rail">
        <div className={styles.empty}>No event selected</div>
      </aside>
    );
  }

  const diff = state?.diff;
  const payloadJSON = state?.payloadJson ?? "";
  let formattedPayload = payloadJSON;
  try {
    formattedPayload = JSON.stringify(JSON.parse(payloadJSON), null, 2);
  } catch {
    // keep as-is
  }

  return (
    <aside className={styles.rail} aria-label="Event detail" data-testid="event-detail-rail">
      {/* Section 1: Kind + entity + timestamp */}
      <section className={styles.section}>
        <div className={styles.kindRow}>
          <span className={styles.kindChip} data-testid="kind-chip">{step.kind}</span>
          <span className={styles.entityName}>{step.entityId}</span>
        </div>
        <dl className={styles.dl}>
          <dt>Timestamp</dt>
          <dd><code className={styles.mono}>{formatTimestamp(step.occurredAt)}</code></dd>
          <dt>Seq</dt>
          <dd><code className={styles.mono}>{step.seq}</code></dd>
        </dl>
      </section>

      {/* Section 2: Diff */}
      {diff && diff.entityDiffs && diff.entityDiffs.length > 0 && (
        <section className={styles.section}>
          <h3 className={styles.sectionHeading} data-testid="diff-heading">Diff</h3>
          {diff.entityDiffs.map((ed) => (
            <div key={ed.entityId}>
              <div className={styles.diffEntityId}>{ed.entityId}</div>
              <dl className={styles.dl}>
                {(ed.fieldDiffs ?? []).map((fd) => (
                  <div key={fd.field} className={styles.diffRow}>
                    <dt className={styles.diffField}>{fd.field}</dt>
                    <dd className={styles.diffValue}>
                      <span className={styles.diffWas}>{fd.was || "—"}</span>
                      {" → "}
                      <span className={styles.diffNow}>{fd.now}</span>
                    </dd>
                  </div>
                ))}
              </dl>
            </div>
          ))}
        </section>
      )}

      {/* Section 3: Identity */}
      <section className={styles.section}>
        <h3 className={styles.sectionHeading} data-testid="identity-heading">Identity</h3>
        <dl className={styles.dl}>
          <dt>event_id</dt>
          <dd><code className={styles.mono} data-testid="event-id-value">{state?.eventId || step.eventId}</code></dd>
          <dt>causation_id</dt>
          <dd><code className={styles.mono}>{state?.causationId || step.causationId || "—"}</code></dd>
          <dt>correlation_id</dt>
          <dd><code className={styles.mono}>{state?.correlationId || "—"}</code></dd>
        </dl>
      </section>

      {/* Section 4: Source */}
      <section className={styles.section}>
        <h3 className={styles.sectionHeading} data-testid="source-heading">Source</h3>
        <dl className={styles.dl}>
          <dt>emitter</dt>
          <dd><code className={styles.mono}>{state?.emitter || "—"}</code></dd>
          <dt>span_id</dt>
          <dd><code className={styles.mono}>{state?.spanId || "—"}</code></dd>
        </dl>
      </section>

      {/* Section 5: Payload */}
      {formattedPayload && (
        <section className={styles.section}>
          <h3 className={styles.sectionHeading} data-testid="payload-heading">Payload</h3>
          <pre
            className={styles.payloadBlock}
            data-testid="payload-block"
            dangerouslySetInnerHTML={{ __html: highlightJSON(formattedPayload) }}
          />
        </section>
      )}
    </aside>
  );
}
