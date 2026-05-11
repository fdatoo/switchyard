// web/src/pkl-editor/index.ts
// Barrel export for all public pkl-editor components and utilities.
export { default as Monaco } from "./Monaco";
export { default as FileTree } from "./FileTree";
export { default as Inspector } from "./Inspector";
export { default as AstBreadcrumb } from "./AstBreadcrumb";
export { default as StatusBar } from "./StatusBar";
export { default as MergeView } from "./merge-view";
export { findStarlarkRegions, lineInStarlarkRegion } from "./embedded";
export { buildDecorations } from "./form-bound-decorations";
export { registerPklLanguage, PKL_LANGUAGE_ID } from "./languages/pkl";
export {
  registerStarlarkLanguage,
  STARLARK_LANGUAGE_ID,
} from "./languages/starlark";
export type { MonacoProps } from "./Monaco";
export type { FileEntry } from "./FileTree";
export type { FormBoundRegion, MonacoDecoration } from "./form-bound-decorations";
export type { StarlarkRegion } from "./embedded";
