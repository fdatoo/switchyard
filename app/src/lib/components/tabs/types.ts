import type { IconName } from "@/lib/components/icon/SyIcon.vue";

/**
 * One tab descriptor. The full tab strip is configured by passing an array
 * of these — `<SyTabs :tabs="[…]" />` — rather than child components, so
 * tabs can be data-driven from RPCs and serialized to/from URL search
 * params without composition gymnastics.
 */
export interface TabDef {
  /** Stable identifier; also the v-model value when this tab is selected. */
  id: string;
  /** Visible label. */
  label: string;
  /** Optional leading icon. */
  icon?: IconName;
  /** Optional trailing count badge. */
  badge?: {
    count: number | string;
    intent?: "neutral" | "accent" | "good" | "warn" | "bad" | "info" | "purple";
  };
  /** Visually dim + non-interactive. */
  disabled?: boolean;
}

export type TabsSize = "sm" | "md" | "lg";
