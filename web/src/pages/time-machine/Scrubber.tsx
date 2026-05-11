import styles from "./Scrubber.module.css";
import type { ChainEvent } from "../../data/replay-client";

export type PlaySpeed = 0.25 | 1 | 2 | 4;

const SPEEDS: PlaySpeed[] = [0.25, 1, 2, 4];

/** Color-code event kinds for the track dots. */
function dotColor(kind: string): string {
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

export interface ScrubberProps {
  steps: ChainEvent[];
  currentIndex: number;
  playing: boolean;
  speed: PlaySpeed;
  onPlay: () => void;
  onPause: () => void;
  onNext: () => void;
  onPrev: () => void;
  onFirst: () => void;
  onLast: () => void;
  onSeek: (index: number) => void;
  onSpeedChange: (speed: PlaySpeed) => void;
}

/**
 * Scrubber — transport controls + dot track for the Time-machine.
 */
export function Scrubber({
  steps,
  currentIndex,
  playing,
  speed,
  onPlay,
  onPause,
  onNext,
  onPrev,
  onFirst,
  onLast,
  onSeek,
  onSpeedChange,
}: ScrubberProps) {
  const total = steps.length;
  const current = steps[currentIndex];
  const seq = current?.seq ?? "—";
  const ts = current?.occurredAt ? formatTimestamp(current.occurredAt) : "—";
  const posLabel = total > 0 ? `step ${currentIndex + 1} of ${total}` : "no steps";

  return (
    <div className={styles.scrubber} data-testid="scrubber">
      {/* Transport controls */}
      <div className={styles.transport}>
        <button className={styles.transportBtn} onClick={onFirst} aria-label="First step" disabled={total === 0}>
          ⏮
        </button>
        <button className={styles.transportBtn} onClick={onPrev} aria-label="Previous step" disabled={currentIndex === 0 || total === 0}>
          ‹
        </button>
        {playing ? (
          <button className={styles.transportBtn} onClick={onPause} aria-label="Pause" data-testid="pause-btn">
            ⏸
          </button>
        ) : (
          <button className={styles.transportBtn} onClick={onPlay} aria-label="Play" data-testid="play-btn" disabled={total === 0}>
            ▶
          </button>
        )}
        <button
          className={styles.transportBtn}
          onClick={onNext}
          aria-label="Next step"
          data-testid="next-btn"
          disabled={currentIndex >= total - 1 || total === 0}
        >
          ›
        </button>
        <button className={styles.transportBtn} onClick={onLast} aria-label="Last step" disabled={total === 0}>
          ⏭
        </button>
      </div>

      {/* Position label */}
      <span className={styles.posLabel} data-testid="pos-label">
        {posLabel}
        {seq !== "—" && (
          <>
            {" · "}
            <code className={styles.seq}>seq {seq}</code>
            {" · "}
            <code className={styles.ts}>{ts}</code>
          </>
        )}
      </span>

      {/* Dot track */}
      <div className={styles.track} role="slider" aria-label="Event track" aria-valuemin={0} aria-valuemax={total - 1} aria-valuenow={currentIndex}>
        {steps.map((step, i) => {
          const pct = total > 1 ? (i / (total - 1)) * 100 : 50;
          const isActive = i === currentIndex;
          return (
            <button
              key={step.seq + step.eventId}
              className={`${styles.dot} ${isActive ? styles.dotActive : ""}`}
              style={{
                left: `${pct}%`,
                backgroundColor: isActive ? "var(--sy-color-accent)" : dotColor(step.kind),
              }}
              onClick={() => onSeek(i)}
              aria-label={`Step ${i + 1}: ${step.kind}`}
              data-testid={`dot-${i}`}
            />
          );
        })}
        {/* Accent position bar */}
        {total > 0 && (
          <div
            className={styles.posBar}
            style={{ left: `${total > 1 ? (currentIndex / (total - 1)) * 100 : 50}%` }}
            aria-hidden="true"
          />
        )}
      </div>

      {/* Speed selector */}
      <div className={styles.speedGroup} role="group" aria-label="Playback speed">
        {SPEEDS.map((s) => (
          <button
            key={s}
            className={`${styles.speedBtn} ${speed === s ? styles.speedBtnActive : ""}`}
            onClick={() => onSpeedChange(s)}
            aria-label={`${s}× speed`}
            aria-pressed={speed === s}
          >
            {s}×
          </button>
        ))}
      </div>
    </div>
  );
}
