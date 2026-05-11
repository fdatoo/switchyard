import styles from "./StatePane.module.css";
import type { EntityState, StateDiff } from "../../data/replay-client";

export type StateMode = "all" | "affected" | "diff";

const MODES: { key: StateMode; label: string }[] = [
  { key: "all", label: "All entities" },
  { key: "affected", label: "Affected only" },
  { key: "diff", label: "Diff from prev" },
];

/** Gradient for entity kind icon slot. */
function entityIconStyle(entityId: string): React.CSSProperties {
  if (entityId.startsWith("light.") || entityId.startsWith("light/")) {
    return {
      background: "linear-gradient(135deg, var(--sy-color-warn) 0%, var(--sy-color-info) 100%)",
    };
  }
  if (entityId.startsWith("climate.") || entityId.startsWith("climate/")) {
    return {
      background: "linear-gradient(135deg, var(--sy-color-info) 0%, var(--sy-color-good) 100%)",
    };
  }
  return {
    background: "var(--sy-color-surface-3)",
  };
}

export interface StatePaneProps {
  entities: EntityState[];
  diff: StateDiff;
  whyInteresting: string;
  mode: StateMode;
  onModeChange: (mode: StateMode) => void;
}

/**
 * StatePane — center pane with three view modes: All, Affected only, Diff from prev.
 */
export function StatePane({ entities, diff, whyInteresting, mode, onModeChange }: StatePaneProps) {
  // Build a set of changed entity IDs from the diff.
  const changedEntityIds = new Set<string>(
    (diff.entityDiffs ?? []).map((ed) => ed.entityId),
  );

  // Build a map for diff lookup: entityId -> { field -> {was, now} }
  type DiffMap = Record<string, Record<string, { was: string; now: string }>>;
  const diffMap: DiffMap = {};
  for (const ed of diff.entityDiffs ?? []) {
    diffMap[ed.entityId] = {};
    for (const fd of ed.fieldDiffs ?? []) {
      diffMap[ed.entityId][fd.field] = { was: fd.was, now: fd.now };
    }
  }

  // Determine which entities to display based on mode.
  let displayEntities: EntityState[];
  if (mode === "all") {
    displayEntities = entities;
  } else {
    // "affected" and "diff" show only changed entities
    displayEntities = entities.filter((e) => changedEntityIds.has(e.entityId));
  }

  return (
    <main className={styles.pane} data-testid="state-pane">
      {/* Mode segment control */}
      <div className={styles.modeControl} role="group" aria-label="View mode">
        {MODES.map(({ key, label }) => (
          <button
            key={key}
            className={`${styles.modeBtn} ${mode === key ? styles.modeBtnActive : ""}`}
            onClick={() => onModeChange(key)}
            aria-pressed={mode === key}
            data-testid={`mode-btn-${key}`}
          >
            {label}
          </button>
        ))}
      </div>

      {/* Entity list */}
      <div className={styles.entityList}>
        {displayEntities.length === 0 ? (
          <div className={styles.emptyState}>
            {mode === "affected" || mode === "diff"
              ? "No changes at this step"
              : "No entities in snapshot"}
          </div>
        ) : (
          displayEntities.map((entity) => {
            const isChanged = changedEntityIds.has(entity.entityId);
            const fieldDiffs = diffMap[entity.entityId] ?? {};

            return (
              <div
                key={entity.entityId}
                className={`${styles.entityCard} ${isChanged ? styles.entityCardChanged : ""}`}
                data-testid={`entity-card-${entity.entityId}`}
              >
                {/* Icon + header */}
                <div className={styles.entityHeader}>
                  <div className={styles.entityIcon} style={entityIconStyle(entity.entityId)} aria-hidden="true" />
                  <code className={styles.entityId}>{entity.entityId}</code>
                  {isChanged && (
                    <span className={styles.changedLabel} data-testid={`changed-label-${entity.entityId}`}>
                      changed this step
                    </span>
                  )}
                </div>

                {/* Field grid */}
                <dl className={styles.fieldGrid}>
                  {Object.entries(entity.fields ?? {}).map(([field, value]) => {
                    const fd = fieldDiffs[field];
                    const hasDiff = mode === "diff" && fd !== undefined;
                    return (
                      <div key={field} className={styles.fieldRow}>
                        <dt className={styles.fieldKey}>{field}</dt>
                        <dd className={styles.fieldValue}>
                          {hasDiff ? (
                            <>
                              <span className={styles.diffWas}>{fd.was || "—"}</span>
                              {" → "}
                              <span className={styles.diffNow}>{fd.now}</span>
                            </>
                          ) : (
                            value
                          )}
                        </dd>
                      </div>
                    );
                  })}
                </dl>
              </div>
            );
          })
        )}
      </div>

      {/* "Why interesting?" panel */}
      {whyInteresting && (
        <div className={styles.whyPanel} data-testid="why-interesting-panel">
          <h3 className={styles.whyHeading}>Why is this step interesting?</h3>
          <p className={styles.whyText}>{whyInteresting}</p>
        </div>
      )}
    </main>
  );
}
