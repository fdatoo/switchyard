<!--
  SyDataTable — sortable data table.

  Developer language's primary list affordance per the vision spec (Rooms
  as a sortable table instead of cards). Friendly/ambient surfaces
  generally use SyListRow stacks instead; the table is here for any
  surface that needs columnar comparison.

  Data-driven: `columns` (definitions) + `rows` (data). The table never
  mutates rows — sorting is "controlled" via `v-model:sort` (consumer
  applies the sort to its own data, server-side or client-side). The
  component just emits sort intent on header clicks.

  Cell rendering precedence:
    1. Named slot `#cell-<column.id>` — for rich content (badges, icons).
    2. `column.cell(row)` — function returning string/number.
    3. `row[column.id]` — direct lookup.

  Empty state: when `rows.length === 0`, renders the `empty` slot if
  provided, otherwise a SyEmptyState with sensible defaults. Loading state
  is consumer-responsibility — pass `loading=true` and we'll show a
  spinner-style SyEmptyState in the body region.

  Filters: the table provides a `toolbar` slot above the header row but
  inside the bordered container. Consumers compose their own filter UI
  there — a SySearchInput, SyFilterChip stack, a "+ Filter" SyMenu, row
  counts, action buttons. The table doesn't manage filter state because
  filtering can be local (in-memory) or remote (RPC) and the right model
  depends on the consumer.
-->
<script setup lang="ts" generic="T extends object">
import { computed } from "vue";
import SyText from "@/lib/components/text/SyText.vue";
import SyIcon from "@/lib/components/icon/SyIcon.vue";
import SyEmptyState from "@/lib/components/empty-state/SyEmptyState.vue";
import type { ColumnDef, Density, SortState } from "./types";

const props = withDefaults(
  defineProps<{
    columns: ColumnDef<T>[];
    rows: T[];
    /** Function returning a stable key per row. Defaults to row.id if present. */
    rowKey?: (row: T) => string | number;
    /** Sort state for v-model:sort. Optional. */
    sort?: SortState | null;
    density?: Density;
    /** Show a loading empty state instead of rows. */
    loading?: boolean;
    /** Empty-state title when `rows.length === 0` and not loading. */
    emptyTitle?: string;
    emptyDescription?: string;
  }>(),
  {
    density: "comfortable",
    loading: false,
    emptyTitle: "No data",
    emptyDescription: "",
  },
);

const emit = defineEmits<{
  "update:sort": [value: SortState | null];
  "row-click": [row: T];
}>();

function defaultRowKey(row: T): string | number {
  const id = (row as { id?: unknown }).id;
  if (typeof id === "string" || typeof id === "number") return id;
  /* Fall back to JSON; not stable across re-renders for mutated rows, but
     fine when rows are immutable per render. */
  return JSON.stringify(row);
}

const getRowKey = computed(() => props.rowKey ?? defaultRowKey);

function onHeaderClick(col: ColumnDef<T>): void {
  if (!col.sortable) return;
  const current = props.sort;
  if (!current || current.columnId !== col.id) {
    emit("update:sort", { columnId: col.id, direction: "asc" });
  } else if (current.direction === "asc") {
    emit("update:sort", { columnId: col.id, direction: "desc" });
  } else {
    /* Third click clears the sort. */
    emit("update:sort", null);
  }
}

function cellValue(row: T, col: ColumnDef<T>): string | number | null | undefined {
  if (col.cell) return col.cell(row);
  const v = (row as Record<string, unknown>)[col.id];
  if (v == null) return null;
  if (typeof v === "string" || typeof v === "number") return v;
  return String(v);
}

function colWidth(col: ColumnDef<T>): string | undefined {
  if (col.width == null) return undefined;
  return typeof col.width === "number" ? `${col.width}px` : col.width;
}

const tableClass = computed(() => [
  "sy-table",
  `sy-table--${props.density}`,
]);

const isEmpty = computed(() => !props.loading && props.rows.length === 0);
</script>

<template>
  <div class="sy-table__wrap">
    <div v-if="$slots.toolbar" class="sy-table__toolbar">
      <slot name="toolbar" />
    </div>
    <table :class="tableClass">
      <thead>
        <tr>
          <th
            v-for="col in columns"
            :key="col.id"
            :class="[
              `sy-table__th--align-${col.align ?? 'left'}`,
              col.sortable && 'sy-table__th--sortable',
              col.sortable && sort?.columnId === col.id && 'sy-table__th--sorted',
            ]"
            :style="{ width: colWidth(col) }"
            :aria-sort="
              col.sortable && sort?.columnId === col.id
                ? sort.direction === 'asc' ? 'ascending' : 'descending'
                : col.sortable ? 'none' : undefined
            "
            @click="onHeaderClick(col)"
          >
            <span class="sy-table__th-inner">
              <SyText variant="label" tone="subtle" as="span">{{ col.header }}</SyText>
              <span v-if="col.sortable" class="sy-table__sort" aria-hidden="true">
                <SyIcon
                  v-if="sort?.columnId === col.id"
                  :name="sort.direction === 'asc' ? 'chevron-down' : 'chevron-down'"
                  :size="12"
                  :style="sort.direction === 'asc' ? 'transform: rotate(180deg)' : ''"
                />
                <SyIcon v-else name="chevron-down" :size="12" style="opacity: 0.3" />
              </span>
            </span>
          </th>
        </tr>
      </thead>
      <tbody v-if="!loading && !isEmpty">
        <tr
          v-for="row in rows"
          :key="getRowKey(row)"
          class="sy-table__row"
          @click="emit('row-click', row)"
        >
          <td
            v-for="col in columns"
            :key="col.id"
            :class="[`sy-table__td--align-${col.align ?? 'left'}`]"
          >
            <slot :name="`cell-${col.id}`" :row="row" :value="cellValue(row, col)">
              {{ cellValue(row, col) }}
            </slot>
          </td>
        </tr>
      </tbody>
    </table>

    <div v-if="loading" class="sy-table__placeholder">
      <SyEmptyState loading title="Loading…" />
    </div>
    <div v-else-if="isEmpty" class="sy-table__placeholder">
      <slot name="empty">
        <SyEmptyState :title="emptyTitle" :description="emptyDescription" />
      </slot>
    </div>
  </div>
</template>

<style scoped>
.sy-table__wrap {
  width: 100%;
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius);
  overflow: hidden;
  background: var(--sy-color-surface-1);
}

.sy-table {
  width: 100%;
  border-collapse: collapse;
  font-family: var(--sy-font-body);
}

.sy-table thead th {
  position: sticky;
  top: 0;
  background: var(--sy-color-surface-2);
  border-bottom: 1px solid var(--sy-color-line);
  text-align: left;
  font-weight: 500;
  padding: var(--sy-space-2) var(--sy-space-3);
  user-select: none;
  white-space: nowrap;
}
.sy-table__th--sortable { cursor: pointer; }
.sy-table__th--sortable:hover { background: var(--sy-color-surface-3); }
.sy-table__th--sorted .sy-table__sort { color: var(--sy-color-accent); }

.sy-table__th--align-left   { text-align: left; }
.sy-table__th--align-right  { text-align: right; }
.sy-table__th--align-center { text-align: center; }

.sy-table__th-inner {
  display: inline-flex;
  align-items: center;
  gap: var(--sy-space-1);
}

.sy-table__sort {
  display: inline-flex;
  align-items: center;
  color: var(--sy-color-fg-4);
  transition: color var(--sy-motion-fast), transform var(--sy-motion-fast);
}

.sy-table tbody td {
  padding: var(--sy-space-2) var(--sy-space-3);
  border-bottom: 1px solid var(--sy-color-line-soft);
  color: var(--sy-color-fg);
  font-size: 0.8125rem;
  vertical-align: middle;
}
.sy-table tbody tr:last-child td { border-bottom: 0; }

.sy-table__td--align-left   { text-align: left; }
.sy-table__td--align-right  { text-align: right; font-family: var(--sy-font-numeric); font-feature-settings: var(--sy-numeric-feature); }
.sy-table__td--align-center { text-align: center; }

.sy-table__row {
  transition: background var(--sy-motion-fast);
}
.sy-table__row:hover td {
  background: var(--sy-color-surface-2);
}

/* Density */
.sy-table--compact thead th,
.sy-table--compact tbody td {
  padding: 4px var(--sy-space-3);
  font-size: 0.75rem;
}
.sy-table--comfortable thead th,
.sy-table--comfortable tbody td {
  padding: var(--sy-space-2) var(--sy-space-3);
}

.sy-table__placeholder {
  padding: var(--sy-space-4);
}

.sy-table__toolbar {
  display: flex;
  align-items: center;
  gap: var(--sy-space-2);
  flex-wrap: wrap;
  padding: var(--sy-space-2) var(--sy-space-3);
  border-bottom: 1px solid var(--sy-color-line);
  background: var(--sy-color-surface-1);
}
</style>
