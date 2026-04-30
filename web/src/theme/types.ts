export type ThemeMode = "light" | "dark";
export type ThemeModePreference = ThemeMode | "system";
export type LanguageId = "developer";

export type MotionPreset = {
  type: "tween" | "spring";
  duration?: number;
  ease?: number[];
  damping?: number;
  stiffness?: number;
};

export type TokenSet = {
  color: {
    bg: string; surface1: string; surface2: string; border: string;
    fg: string; fgMuted: string; accent: string;
    success: string; warning: string; danger: string;
  };
  radius: { sm: string; md: string; lg: string; pill: string };
  motion: { snappy: MotionPreset; spring: MotionPreset; slow: MotionPreset };
  font: { display: string; body: string; numeric: string };
};

export type LanguagePreset = {
  id: LanguageId;
  modes: { light: TokenSet; dark: TokenSet };
};

export type Theme = {
  language: LanguageId;
  mode: ThemeMode;
  tokens: TokenSet;
};
