// web/src/pkl-editor/form-bound-decorations.ts
// Builds Monaco editor decorations for form-bound regions returned by
// ConfigService.GetFormBoundRegions (added in Plan 11).

export interface FormBoundRegion {
  startLine: number;
  endLine: number;
  formEditorId: string; // e.g. "automations/sunset-lights.pkl"
  label: string; // e.g. "actions[0]"
}

export interface MonacoDecoration {
  range: {
    startLineNumber: number;
    startColumn: number;
    endLineNumber: number;
    endColumn: number;
  };
  options: {
    className?: string;
    glyphMarginClassName?: string;
    glyphMarginHoverMessage?: { value: string };
    isWholeLine?: boolean;
    overviewRuler?: { color: string; position: number };
  };
}

export function buildDecorations(
  regions: FormBoundRegion[]
): MonacoDecoration[] {
  return regions.map((r) => ({
    range: {
      startLineNumber: r.startLine,
      startColumn: 1,
      endLineNumber: r.endLine,
      endColumn: Number.MAX_SAFE_INTEGER,
    },
    options: {
      className: "form-bound-region", // purple tinted background; defined in pkl-editor.css
      glyphMarginClassName: "form-bound-glyph", // purple bar in the gutter
      glyphMarginHoverMessage: {
        value: `**Form-bound region** — _${r.label}_\n\n[Reveal in form editor →](action:revealFormEditor?${r.formEditorId})`,
      },
      isWholeLine: true,
      overviewRuler: {
        color: "var(--sy-color-purple)",
        position: 4 /* OverviewRulerLane.Right */,
      },
    },
  }));
}
