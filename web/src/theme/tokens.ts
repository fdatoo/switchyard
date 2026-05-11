/**
 * Typed token name exports for the --sy-* design token surface.
 * Components must reference these names rather than raw strings to stay
 * consistent with the ESLint no-raw-tokens rule.
 */

export const colors = [
  "--sy-color-bg",
  "--sy-color-surface-1",
  "--sy-color-surface-2",
  "--sy-color-surface-3",
  "--sy-color-sidebar",
  "--sy-color-line",
  "--sy-color-line-soft",
  "--sy-color-fg",
  "--sy-color-fg-2",
  "--sy-color-fg-3",
  "--sy-color-fg-4",
  "--sy-color-fg-5",
  "--sy-color-accent",
  "--sy-color-accent-2",
  "--sy-color-accent-soft",
  "--sy-color-good",
  "--sy-color-warn",
  "--sy-color-bad",
  "--sy-color-info",
  "--sy-color-purple",
] as const;

export const radii = [
  "--sy-radius-sm",
  "--sy-radius",
  "--sy-radius-lg",
  "--sy-radius-xl",
  "--sy-radius-pill",
] as const;

export const spaces = [
  "--sy-space-1",
  "--sy-space-2",
  "--sy-space-3",
  "--sy-space-4",
  "--sy-space-5",
  "--sy-space-6",
] as const;

export const fonts = [
  "--sy-font-display",
  "--sy-font-body",
  "--sy-font-numeric",
] as const;

export const motions = [
  "--sy-motion-fast",
  "--sy-motion",
  "--sy-motion-slow",
  "--sy-motion-spring",
] as const;

export const shadows = [
  "--sy-shadow",
  "--sy-shadow-2",
  "--sy-shadow-elevated",
] as const;

export const gradients = ["--sy-gradient-tod"] as const;

export type ColorToken = (typeof colors)[number];
export type RadiusToken = (typeof radii)[number];
export type SpaceToken = (typeof spaces)[number];
export type FontToken = (typeof fonts)[number];
export type MotionToken = (typeof motions)[number];
export type ShadowToken = (typeof shadows)[number];
export type GradientToken = (typeof gradients)[number];

export type Token =
  | ColorToken
  | RadiusToken
  | SpaceToken
  | FontToken
  | MotionToken
  | ShadowToken
  | GradientToken;

/** Helper: get the computed value of a --sy-* token from documentElement */
export function getToken(name: Token): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

/** Helper: CSS var() reference */
export function syVar(name: Token): string {
  return `var(${name})`;
}
