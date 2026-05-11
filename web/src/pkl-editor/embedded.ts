// web/src/pkl-editor/embedded.ts
// Detects starlark("…") and starlark("""…""") regions within a Pkl source string.
// Returns an array of line ranges (1-based) for the Starlark content inside the call.

export interface StarlarkRegion {
  /** 1-based line of the first Starlark source line (after the opening quote) */
  startLine: number;
  /** 1-based line of the last Starlark source line (before the closing quote) */
  endLine: number;
  /** Character offset of the Starlark content start within the full source */
  startOffset: number;
  /** Character offset of the Starlark content end within the full source */
  endOffset: number;
}

// Matches starlark(""" or starlark(r""" variants.
const OPEN_TRIPLE = /starlark\s*\(\s*(?:r?""")/g;
const CLOSE_TRIPLE = /"""\s*\)/g;

/**
 * Scan the Pkl source for `starlark("""...""")` regions and return
 * the 1-based line ranges for the embedded Starlark content.
 *
 * Only triple-quoted forms are handled (most common in Pkl).
 * Single-quoted forms are uncommon enough that they are intentionally out of scope.
 */
export function findStarlarkRegions(source: string): StarlarkRegion[] {
  const regions: StarlarkRegion[] = [];
  const lines = source.split("\n");

  function offsetToLine(offset: number): number {
    let remaining = offset;
    for (let i = 0; i < lines.length; i++) {
      if (remaining <= lines[i].length) return i + 1;
      remaining -= lines[i].length + 1;
    }
    return lines.length;
  }

  // Try triple-quoted first (most common in Pkl Starlark usage).
  OPEN_TRIPLE.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = OPEN_TRIPLE.exec(source)) !== null) {
    const contentStart = m.index + m[0].length;
    CLOSE_TRIPLE.lastIndex = contentStart;
    const close = CLOSE_TRIPLE.exec(source);
    if (!close) continue;
    const contentEnd = close.index;
    regions.push({
      startLine: offsetToLine(contentStart),
      endLine: offsetToLine(contentEnd),
      startOffset: contentStart,
      endOffset: contentEnd,
    });
    OPEN_TRIPLE.lastIndex = close.index + close[0].length;
  }

  return regions;
}

/** Returns true if the given 1-based line falls inside any Starlark region. */
export function lineInStarlarkRegion(
  regions: StarlarkRegion[],
  line: number
): boolean {
  return regions.some((r) => line >= r.startLine && line <= r.endLine);
}
