import { useEffect, useMemo, useState } from "react";
import type { KeyboardEvent, MouseEvent, ReactNode } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown, Columns3 } from "lucide-react";
import { cn } from "cn";
import { Button } from "@/shared/components/primitives/button";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import { Popover, PopoverContent, PopoverTrigger } from "@/shared/components/primitives/popover";
import { EmptyState } from "@/shared/components/ui/empty-state";
import { ErrorState } from "@/shared/components/ui/error-state";
import { TableSkeleton } from "@/shared/components/ui/skeletons";
import { onHiddenColumnsChange, readHiddenColumns, writeHiddenColumns } from "@/shared/lib/table-columns";

export type DataTableAlign = "left" | "center" | "right";
export type SortDir = "asc" | "desc";
export interface DataTableSort {
  key: string;
  dir: SortDir;
}

export interface DataTableColumn<T> {
  header: string;
  render: (row: T) => ReactNode;
  /** Stable id for the column chooser (default: header). */
  id?: string;
  width?: string | number;
  /** Numbers and money: "right". */
  align?: DataTableAlign;
  /** Server-side sort key; makes the header clickable. */
  sortKey?: string;
  /** Hidden until the user shows it from the "Kolom" menu. */
  defaultHidden?: boolean;
}

export interface DataTableProps<T> {
  columns: DataTableColumn<T>[];
  /** null/undefined is empty (a Go API sends null for an empty slice). */
  rows: T[] | null | undefined;
  rowKey: (row: T) => string | number;
  /** Skeleton while there are no rows yet; dimmed while reloading. */
  loading?: boolean;
  /** A failed load — the API's message, with "Coba lagi" when onRetry is given. */
  error?: string | null;
  onRetry?: () => void;
  emptyTitle?: string;
  emptyDescription?: ReactNode;
  /** E.g. "Buat produk pertama". */
  emptyAction?: ReactNode;
  /** The whole row opens the record (clicks on links, buttons, and inputs inside are ignored). */
  onRowClick?: (row: T) => void;
  /** Accessible name of a clickable row, e.g. "Buka SO-2026-00012". */
  rowLabel?: (row: T) => string;
  sort?: DataTableSort | null;
  onSortChange?: (sort: DataTableSort) => void;
  /** A checkbox per row and a bulk-action bar. */
  selectable?: boolean;
  selectedKeys?: ReadonlySet<string | number>;
  onSelectionChange?: (keys: Set<string | number>) => void;
  /** The bulk-action bar's content; replaces the toolbar while rows are selected. */
  bulkActions?: (selected: ReadonlySet<string | number>) => ReactNode;
  stickyHeader?: boolean;
  /** Height before the table scrolls inside itself. */
  maxHeight?: string;
  /** Shows the "Kolom" chooser and remembers the choice under this key. */
  columnsKey?: string;
  /** Accessible name of the table. */
  caption?: string;
}

const ALIGN: Record<DataTableAlign, string> = { left: "text-left", center: "text-center", right: "text-right" };

// Includes portal content (a ⋯ menu, a dialog): React events bubble through the component tree to <tr>.
const INTERACTIVE =
  "a,button,input,select,textarea,label,[role=button],[role=checkbox],[role=menu],[role=menuitem],[role=dialog],[role=alertdialog],[role=listbox],[role=option]";

const colId = <T,>(c: DataTableColumn<T>) => c.id ?? c.header;

function initialHidden<T>(key: string | undefined, columns: DataTableColumn<T>[]): Set<string> {
  const saved = key ? readHiddenColumns(key) : null;
  return new Set(saved ?? columns.filter((c) => c.defaultHidden).map(colId));
}

/**
 * The list table: sticky header, sortable columns, density-aware rows, clickable rows, selection
 * with a bulk-action bar, a column chooser, and built-in loading, empty, and error states.
 * Pagination and filters sit around it (Pagination, Toolbar).
 */
export function DataTable<T>({
  columns,
  rows: rowsProp,
  rowKey,
  loading = false,
  error,
  onRetry,
  emptyTitle = "Belum ada data",
  emptyDescription,
  emptyAction,
  onRowClick,
  rowLabel,
  sort,
  onSortChange,
  selectable = false,
  selectedKeys,
  onSelectionChange,
  bulkActions,
  stickyHeader = true,
  maxHeight = "70vh",
  columnsKey,
  caption,
}: DataTableProps<T>) {
  const rows = rowsProp ?? [];
  const [hidden, setHidden] = useState<Set<string>>(() => initialHidden(columnsKey, columns));
  useEffect(() => {
    if (!columnsKey) return;
    return onHiddenColumnsChange(columnsKey, () => setHidden(initialHidden(columnsKey, columns)));
    // columns is a new array every render; its defaults are read only when the event arrives.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [columnsKey]);
  const visible = useMemo(() => columns.filter((c) => !hidden.has(colId(c))), [columns, hidden]);

  if (error) return <ErrorState message={error} onRetry={onRetry} className="m-4" />;
  if (loading && rows.length === 0) return <TableSkeleton cols={Math.min(columns.length, 6)} />;
  if (rows.length === 0) return <EmptyState title={emptyTitle} description={emptyDescription} action={emptyAction} />;

  const selected = selectedKeys ?? new Set<string | number>();
  const allSelected = rows.every((r) => selected.has(rowKey(r)));
  const someSelected = !allSelected && rows.some((r) => selected.has(rowKey(r)));

  function toggleAll(checked: boolean) {
    const next = new Set(selected);
    for (const r of rows) {
      if (checked) next.add(rowKey(r));
      else next.delete(rowKey(r));
    }
    onSelectionChange?.(next);
  }

  function toggleOne(key: string | number, checked: boolean) {
    const next = new Set(selected);
    if (checked) next.add(key);
    else next.delete(key);
    onSelectionChange?.(next);
  }

  function toggleColumn(id: string) {
    const next = new Set(hidden);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setHidden(next);
    if (columnsKey) writeHiddenColumns(columnsKey, [...next]);
  }

  function rowClick(e: MouseEvent<HTMLTableRowElement>, row: T) {
    if (!onRowClick || (e.target as HTMLElement).closest(INTERACTIVE)) return;
    onRowClick(row);
  }

  function rowKeyDown(e: KeyboardEvent<HTMLTableRowElement>, row: T) {
    if (!onRowClick || e.target !== e.currentTarget || e.key !== "Enter") return;
    e.preventDefault();
    onRowClick(row);
  }

  const hasSelection = selectable && selected.size > 0;
  const showBar = Boolean(columnsKey) || hasSelection;

  return (
    <div className={cn(loading && "opacity-60 transition-opacity")} aria-busy={loading || undefined}>
      {showBar && (
        <div className="flex min-h-12 items-center justify-between gap-3 border-b border-border px-5 py-2">
          {hasSelection ? (
            <div className="flex flex-wrap items-center gap-3">
              <span className="tabular text-sm font-medium text-foreground">{selected.size} dipilih</span>
              {bulkActions?.(selected)}
              <Button variant="ghost" size="sm" onClick={() => onSelectionChange?.(new Set())}>
                Batal pilih
              </Button>
            </div>
          ) : (
            <span />
          )}
          {columnsKey && (
            <Popover>
              <PopoverTrigger asChild>
                <Button variant="outline" size="sm">
                  <Columns3 aria-hidden /> Kolom
                </Button>
              </PopoverTrigger>
              <PopoverContent align="end" className="w-56 p-2">
                <ul className="flex flex-col">
                  {columns
                    .filter((c) => c.header !== "")
                    .map((c) => (
                      <li key={colId(c)}>
                        <label className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent">
                          <Checkbox checked={!hidden.has(colId(c))} onCheckedChange={() => toggleColumn(colId(c))} />
                          {c.header}
                        </label>
                      </li>
                    ))}
                </ul>
              </PopoverContent>
            </Popover>
          )}
        </div>
      )}
      <div className="overflow-auto" style={{ maxHeight }} role="region" aria-label={caption ?? "Tabel data"} tabIndex={0}>
        <table className="w-full min-w-[640px] border-collapse">
          {caption && <caption className="sr-only">{caption}</caption>}
          <thead className={cn(stickyHeader && "sticky top-0 z-10")}>
            <tr className="border-b border-border bg-background">
              {selectable && (
                <th scope="col" className="w-10 py-2.5 pl-5">
                  <Checkbox
                    aria-label="Pilih semua baris"
                    checked={allSelected ? true : someSelected ? "indeterminate" : false}
                    onCheckedChange={(v) => toggleAll(v === true)}
                  />
                </th>
              )}
              {visible.map((col) => {
                const dir = col.sortKey && sort?.key === col.sortKey ? sort.dir : null;
                const SortIcon = dir === "asc" ? ArrowUp : dir === "desc" ? ArrowDown : ArrowUpDown;
                return (
                  <th
                    key={colId(col)}
                    scope="col"
                    aria-sort={dir ? (dir === "asc" ? "ascending" : "descending") : undefined}
                    style={col.width !== undefined ? { width: col.width } : undefined}
                    className={cn(
                      "px-4 py-2.5 text-xs font-semibold tracking-[0.02em] whitespace-nowrap text-muted-foreground first:pl-5 last:pr-5",
                      ALIGN[col.align ?? "left"],
                    )}
                  >
                    {col.sortKey && onSortChange ? (
                      <button
                        type="button"
                        onClick={() => onSortChange({ key: col.sortKey!, dir: dir === "asc" ? "desc" : "asc" })}
                        className={cn("inline-flex items-center gap-1 rounded-sm hover:text-foreground", col.align === "right" && "flex-row-reverse")}
                      >
                        {col.header}
                        <SortIcon size={12} aria-hidden className={dir ? "text-foreground" : "opacity-50"} />
                      </button>
                    ) : (
                      col.header
                    )}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => {
              const key = rowKey(row);
              const isSelected = selected.has(key);
              return (
                <tr
                  key={key}
                  data-state={isSelected ? "selected" : undefined}
                  onClick={onRowClick ? (e) => rowClick(e, row) : undefined}
                  onKeyDown={onRowClick ? (e) => rowKeyDown(e, row) : undefined}
                  tabIndex={onRowClick ? 0 : undefined}
                  aria-label={onRowClick && rowLabel ? rowLabel(row) : undefined}
                  className={cn(
                    "border-b border-border transition-colors last:border-b-0 hover:bg-background focus-visible:bg-background focus-visible:outline-none",
                    isSelected && "bg-accent/60",
                    onRowClick && "cursor-pointer",
                  )}
                >
                  {selectable && (
                    <td className="w-10 py-(--row-py) pl-5">
                      <Checkbox aria-label="Pilih baris" checked={isSelected} onCheckedChange={(v) => toggleOne(key, v === true)} />
                    </td>
                  )}
                  {visible.map((col) => (
                    <td
                      key={colId(col)}
                      className={cn(
                        "px-4 py-(--row-py) text-sm text-foreground first:pl-5 last:pr-5",
                        ALIGN[col.align ?? "left"],
                        col.align === "right" && "tabular",
                      )}
                    >
                      {col.render(row)}
                    </td>
                  ))}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
