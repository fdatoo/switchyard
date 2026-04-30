import type { MotionPreset } from "./types";

export const motion = {
  snappy: { type: "tween", duration: 0.2, ease: [0.4, 0, 0.2, 1] } satisfies MotionPreset,
  spring: { type: "spring", damping: 24, stiffness: 320 } satisfies MotionPreset,
  slow:   { type: "tween", duration: 0.5, ease: [0.16, 1, 0.3, 1] } satisfies MotionPreset,
};
