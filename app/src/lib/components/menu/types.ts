import type { IconName } from "@/lib/components/icon/SyIcon.vue";

/**
 * Items in a SyMenu's `items` array. A union over three shapes — actions,
 * separators, and group headers — keeps the data structure flat (no nested
 * sub-arrays) while still expressing structure.
 */
export type MenuItem =
  | MenuItemAction
  | MenuItemSeparator
  | MenuItemHeader;

export interface MenuItemAction {
  type: "item";
  /** Stable id; passed to the `select` event when chosen. */
  id: string;
  label: string;
  icon?: IconName;
  /** Keyboard shortcut hint. Display-only; consumer handles the binding. */
  shortcut?: string;
  disabled?: boolean;
  /** `danger` tints the row red — for destructive actions. */
  intent?: "default" | "danger";
}

export interface MenuItemSeparator {
  type: "separator";
}

export interface MenuItemHeader {
  type: "header";
  label: string;
}
