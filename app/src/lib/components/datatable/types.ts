/**
 * Shared types for SyDataTable. Generics let consumers preserve their row
 * type through column definitions and slot props.
 */

export interface ColumnDef<T = Record<string, unknown>> {
  /** Stable id; used as the named-slot key (`#cell-<id>`) and sort key. */
  id: string;
  /** Visible header. */
  header: string;
  /** CSS width: "120px", "20%", "max-content", etc. Omit to let the column auto-size. */
  width?: string | number;
  /** Horizontal alignment of body cells. Default `left`. Numeric columns usually want `right`. */
  align?: "left" | "right" | "center";
  /** Show the sort affordance and emit `update:sort` events on click. */
  sortable?: boolean;
  /**
   * Cell renderer for simple text. Falls back to `row[id]` if both `cell`
   * and the named slot are absent. The named slot takes precedence when
   * provided — use it for badges, icons, or any non-string content.
   */
  cell?: (row: T) => string | number | null | undefined;
}

export interface SortState {
  columnId: string;
  direction: "asc" | "desc";
}

export type Density = "compact" | "comfortable";
