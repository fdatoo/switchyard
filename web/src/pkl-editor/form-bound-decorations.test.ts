import { test, expect } from "vitest";
import { buildDecorations } from "./form-bound-decorations";

const REGIONS = [
  { startLine: 5, endLine: 8, formEditorId: "automations/sunset-lights.pkl", label: "actions[0]" },
];

test("returns one decoration per form-bound region", () => {
  const decs = buildDecorations(REGIONS);
  expect(decs).toHaveLength(1);
  expect(decs[0].range.startLineNumber).toBe(5);
  expect(decs[0].range.endLineNumber).toBe(8);
  expect(decs[0].options.className).toContain("form-bound-region");
  expect(decs[0].options.glyphMarginClassName).toContain("form-bound-glyph");
});
