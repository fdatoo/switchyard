/**
 * Core types for the Switchyard theme/language system.
 * The token values themselves live in the CSS language files; these types
 * describe the provider contract and the motion preset shape.
 */

export type Language = "friendly" | "ambient" | "developer";
export type ThemeMode = "light" | "dark" | "system";
export type ResolvedTheme = "friendly-light" | "friendly-dark" | "ambient" | "developer";

export type MotionPreset = {
  type: "tween" | "spring";
  duration?: number;
  ease?: number[];
  damping?: number;
  stiffness?: number;
};
