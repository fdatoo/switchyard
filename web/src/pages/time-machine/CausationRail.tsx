import styles from "./CausationRail.module.css";
import type { ChainEvent } from "../../data/replay-client";

/** Color-code event kind dots (same palette as Scrubber). */
function kindColor(kind: string): string {
  if (kind.startsWith("command")) return "var(--sy-color-info)";
  if (kind.startsWith("state") || kind.includes("state")) return "var(--sy-color-good)";
  if (kind.startsWith("config") || kind.includes("config")) return "var(--sy-color-purple)";
  if (kind.startsWith("err") || kind.includes("error") || kind.includes("fail")) return "var(--sy-color-bad)";
  return "var(--sy-color-fg-4)";
}

function formatTimestamp(iso: string): string {
  if (!iso) return "—";
  try {
    const d = new Date(iso);
    const hh = d.getUTCHours().toString().padStart(2, "0");
    const mm = d.getUTCMinutes().toString().padStart(2, "0");
    const ss = d.getUTCSeconds().toString().padStart(2, "0");
    const ms = d.getUTCMilliseconds().toString().padStart(3, "0");
    return `${hh}:${mm}:${ss}.${ms}`;
  } catch {
    return iso;
  }
}

export interface CausationRailProps {
  steps: ChainEvent[];
  currentIndex: number;
  mode: "event" | "window";
  onSeek: (index: number) => void;
}

/**
 * CausationRail — left panel listing the causation chain or event window.
 * Clicking any row seeks the scrubber to that step.
 */
export function CausationRail({ steps, currentIndex, mode, onSeek }: CausationRailProps) {
  const header = mode === "event" ? "Causation chain" : "Event window";

  return (
    <nav className={styles.rail} aria-label={header} data-testid="causation-rail">
      <div className={styles.railHeader} data-testid="rail-header">{header}</div>
      <ol className={styles.list}>
        {steps.map((step, i) => {
          const isActive = i === currentIndex;
          return (
            <li key={step.eventId + step.seq} className={styles.item}>
              <button
                className={`${styles.row} ${isActive ? styles.rowActive : ""}`}
                onClick={() => onSeek(i)}
                aria-current={isActive ? "step" : undefined}
                data-testid={`rail-row-${i}`}
              >
                {/* Dot */}
                <span
                  className={`${styles.dot} ${isActive ? styles.dotActive : ""}`}
                  style={{ backgroundColor: kindColor(step.kind) }}
                  aria-hidden="true"
                />
                {/* Content */}
                <span className={styles.content}>
                  <code className={styles.kind}>{step.kind}</code>
                  <span className={styles.ts}>{formatTimestamp(step.occurredAt)}</span>
                </span>
              </button>
            </li>
          );
        })}
        {steps.length === 0 && (
          <li className={styles.empty}>No events</li>
        )}
      </ol>
    </nav>
  );
}
