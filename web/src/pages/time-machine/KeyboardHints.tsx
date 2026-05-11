import styles from "./KeyboardHints.module.css";

/**
 * KeyboardHints — bottom bar showing available keyboard shortcuts.
 * Purely presentational.
 */
export function KeyboardHints() {
  return (
    <footer className={styles.hints} data-testid="keyboard-hints">
      <span className={styles.hint}><kbd className={styles.kbd}>Space</kbd> play/pause</span>
      <span className={styles.hint}><kbd className={styles.kbd}>←</kbd><kbd className={styles.kbd}>→</kbd> step</span>
      <span className={styles.hint}><kbd className={styles.kbd}>⇧←</kbd><kbd className={styles.kbd}>⇧→</kbd> jump 1s</span>
      <span className={styles.hint}><kbd className={styles.kbd}>f</kbd> affected only</span>
      <span className={styles.hint}><kbd className={styles.kbd}>d</kbd> diff</span>
      <span className={styles.hint}><kbd className={styles.kbd}>Esc</kbd> exit</span>
    </footer>
  );
}
