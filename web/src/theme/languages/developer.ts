import type { LanguagePreset, TokenSet } from "../types";
import { motion } from "../motion";

const cssVar = (name: string) => `var(${name})`;
const base = {
  radius: { sm: cssVar("--gh-radius-sm"), md: cssVar("--gh-radius-md"), lg: cssVar("--gh-radius-lg"), pill: cssVar("--gh-radius-pill") },
  motion,
  font: { display: cssVar("--gh-font-display"), body: cssVar("--gh-font-body"), numeric: cssVar("--gh-font-numeric") },
};
const colors = {
  bg: cssVar("--gh-color-bg"), surface1: cssVar("--gh-color-surface-1"), surface2: cssVar("--gh-color-surface-2"),
  border: cssVar("--gh-color-border"), fg: cssVar("--gh-color-fg"), fgMuted: cssVar("--gh-color-fg-muted"),
  accent: cssVar("--gh-color-accent"), success: cssVar("--gh-color-success"), warning: cssVar("--gh-color-warning"), danger: cssVar("--gh-color-danger"),
};
const tokens: TokenSet = { color: colors, ...base };
export const developer: LanguagePreset = { id: "developer", modes: { light: tokens, dark: tokens } };
