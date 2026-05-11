import styles from "./TopBar.module.css";
import type { ChainEvent } from "../../data/replay-client";

export interface TopBarProps {
  title: string;
  subtitle: string;
  mode: "event" | "window";
  steps: ChainEvent[];
  onBack: () => void;
  onExportTrace: () => void;
}

/**
 * TopBar — full-width header bar for the Time-machine page.
 *
 * Right buttons:
 *   - "Open causation chain in graph" (disabled Plan 04)
 *   - "Compare to now" (disabled Plan 04)
 *   - "Export trace" (NDJSON browser download)
 */
export function TopBar({ title, subtitle, mode: _mode, steps, onBack, onExportTrace }: TopBarProps) {
  const canExport = steps.length > 0;

  return (
    <header className={styles.topBar} data-testid="time-machine-topbar">
      <button className={styles.backBtn} onClick={onBack} aria-label="Back">
        ← Back
      </button>

      <div className={styles.titleGroup}>
        <span className={styles.title}>{title}</span>
        {subtitle && <span className={styles.subtitle}>{subtitle}</span>}
      </div>

      <div className={styles.actions}>
        <button
          className={styles.actionBtn}
          disabled
          aria-label="Open causation chain in graph (coming soon)"
        >
          Chain graph
        </button>
        <button
          className={styles.actionBtn}
          disabled
          aria-label="Compare to now (coming soon)"
        >
          Compare to now
        </button>
        <button
          className={styles.actionBtn}
          onClick={canExport ? onExportTrace : undefined}
          disabled={!canExport}
          aria-label="Export trace as NDJSON"
        >
          Export trace
        </button>
      </div>
    </header>
  );
}
