import { useState, useEffect, useCallback, useRef } from "react";
import { TopBar } from "./TopBar";
import { Scrubber } from "./Scrubber";
import { CausationRail } from "./CausationRail";
import { StatePane } from "./StatePane";
import { EventDetailRail } from "./EventDetailRail";
import { KeyboardHints } from "./KeyboardHints";
import { useTimeMachineKeys } from "./useTimeMachineKeys";
import type { PlaySpeed } from "./Scrubber";
import type { StateMode } from "./StatePane";
import type { ChainEvent, EntityState, StateDiff, LoadAtSeqResponse } from "../../data/replay-client";
import { causationChain, window as windowQuery, loadAtSeq } from "../../data/replay-client";
import styles from "./TimeMachinePage.module.css";

interface TimeMachinePageProps {
  mode: "event" | "window";
  eventId?: string;
  fromSeq?: string;
  toSeq?: string;
}

const TICK_BASE_MS = 1000;

/**
 * TimeMachinePage — full-screen time-machine layout.
 *
 * Layout (horizontal):
 *   [CausationRail 220px] [StatePane flex] [EventDetailRail 320px]
 * With TopBar (48px) + Scrubber (52px) stacked at top.
 */
export function TimeMachinePage({ mode, eventId, fromSeq, toSeq }: TimeMachinePageProps) {
  const [steps, setSteps] = useState<ChainEvent[]>([]);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState<PlaySpeed>(1);
  const [stateMode, setStateMode] = useState<StateMode>("affected");
  const [currentState, setCurrentState] = useState<LoadAtSeqResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const playTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Load initial step list.
  useEffect(() => {
    setLoading(true);
    setError(null);

    const load = async () => {
      try {
        if (mode === "event" && eventId) {
          const chain = await causationChain(eventId);
          setSteps(chain.events ?? []);
        } else if (mode === "window" && fromSeq && toSeq) {
          const win = await windowQuery(fromSeq, toSeq);
          setSteps(win.events ?? []);
        }
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load events");
      } finally {
        setLoading(false);
      }
    };

    void load();
  }, [mode, eventId, fromSeq, toSeq]);

  // Load state for current step.
  useEffect(() => {
    if (steps.length === 0) return;
    const step = steps[currentIndex];
    if (!step) return;

    void loadAtSeq(step.seq).then(setCurrentState).catch(() => setCurrentState(null));
  }, [steps, currentIndex]);

  // Auto-play ticker.
  useEffect(() => {
    if (!playing) {
      if (playTimerRef.current) clearTimeout(playTimerRef.current);
      return;
    }
    const tick = () => {
      setCurrentIndex((i) => {
        if (i >= steps.length - 1) {
          setPlaying(false);
          return i;
        }
        return i + 1;
      });
      playTimerRef.current = setTimeout(tick, TICK_BASE_MS / speed);
    };
    playTimerRef.current = setTimeout(tick, TICK_BASE_MS / speed);
    return () => {
      if (playTimerRef.current) clearTimeout(playTimerRef.current);
    };
  }, [playing, speed, steps.length]);

  const handleBack = useCallback(() => {
    window.history.back();
  }, []);

  const handleExportTrace = useCallback(() => {
    const ndjson = steps.map((s) => JSON.stringify(s)).join("\n");
    const blob = new Blob([ndjson], { type: "application/x-ndjson" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `trace-${eventId ?? "window"}.ndjson`;
    a.click();
    URL.revokeObjectURL(url);
  }, [steps, eventId]);

  const handleNext = useCallback(() => setCurrentIndex((i) => Math.min(i + 1, steps.length - 1)), [steps.length]);
  const handlePrev = useCallback(() => setCurrentIndex((i) => Math.max(i - 1, 0)), []);
  const handleFirst = useCallback(() => setCurrentIndex(0), []);
  const handleLast = useCallback(() => setCurrentIndex(Math.max(0, steps.length - 1)), [steps.length]);
  const handlePlay = useCallback(() => setPlaying(true), []);
  const handlePause = useCallback(() => setPlaying(false), []);

  const handleJumpForward = useCallback(() => {
    // Jump forward ~1s by skipping steps with timestamps < current + 1s
    if (steps.length === 0) return;
    const currentTs = steps[currentIndex]?.occurredAt;
    if (!currentTs) return handleNext();
    const targetMs = new Date(currentTs).getTime() + 1000;
    let nextIdx = currentIndex;
    for (let i = currentIndex + 1; i < steps.length; i++) {
      if (new Date(steps[i].occurredAt).getTime() >= targetMs) {
        nextIdx = i;
        break;
      }
      nextIdx = i;
    }
    setCurrentIndex(nextIdx);
  }, [steps, currentIndex, handleNext]);

  const handleJumpBack = useCallback(() => {
    if (steps.length === 0) return;
    const currentTs = steps[currentIndex]?.occurredAt;
    if (!currentTs) return handlePrev();
    const targetMs = new Date(currentTs).getTime() - 1000;
    let prevIdx = currentIndex;
    for (let i = currentIndex - 1; i >= 0; i--) {
      if (new Date(steps[i].occurredAt).getTime() <= targetMs) {
        prevIdx = i;
        break;
      }
      prevIdx = i;
    }
    setCurrentIndex(prevIdx);
  }, [steps, currentIndex, handlePrev]);

  const handleToggleAffected = useCallback(() => {
    setStateMode((m) => (m === "affected" ? "all" : "affected"));
  }, []);

  const handleToggleDiff = useCallback(() => {
    setStateMode((m) => (m === "diff" ? "affected" : "diff"));
  }, []);

  useTimeMachineKeys({
    playing,
    onPlay: handlePlay,
    onPause: handlePause,
    onStepForward: handleNext,
    onStepBack: handlePrev,
    onJumpForward: handleJumpForward,
    onJumpBack: handleJumpBack,
    onToggleAffected: handleToggleAffected,
    onToggleDiff: handleToggleDiff,
    onExit: handleBack,
  });

  const entities: EntityState[] = currentState?.entities ?? [];
  const diff: StateDiff = currentState?.diff ?? { entityDiffs: [] };
  const whyInteresting = currentState?.whyInteresting ?? "";
  const currentStep = steps[currentIndex] ?? null;

  const title = mode === "event" ? "Time Machine" : "Event Window";
  const subtitle = mode === "event"
    ? (currentStep?.entityId ?? eventId ?? "")
    : `seq ${fromSeq}–${toSeq}`;

  if (loading) {
    return (
      <div className={styles.page}>
        <div className={styles.loading}>Loading time machine…</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className={styles.page}>
        <div className={styles.error}>Error: {error}</div>
      </div>
    );
  }

  return (
    <div className={styles.page}>
      <TopBar
        title={title}
        subtitle={subtitle}
        mode={mode}
        steps={steps}
        onBack={handleBack}
        onExportTrace={handleExportTrace}
      />
      <Scrubber
        steps={steps}
        currentIndex={currentIndex}
        playing={playing}
        speed={speed}
        onPlay={handlePlay}
        onPause={handlePause}
        onNext={handleNext}
        onPrev={handlePrev}
        onFirst={handleFirst}
        onLast={handleLast}
        onSeek={setCurrentIndex}
        onSpeedChange={setSpeed}
      />
      <div className={styles.body}>
        <CausationRail
          steps={steps}
          currentIndex={currentIndex}
          mode={mode}
          onSeek={setCurrentIndex}
        />
        <StatePane
          entities={entities}
          diff={diff}
          whyInteresting={whyInteresting}
          mode={stateMode}
          onModeChange={setStateMode}
        />
        <EventDetailRail step={currentStep} state={currentState} />
      </div>
      <KeyboardHints />
    </div>
  );
}
