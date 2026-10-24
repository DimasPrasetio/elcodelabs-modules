import type { KeyboardEvent, ReactNode } from "react";
import { cn } from "cn";
import { Skeleton } from "@/shared/components/primitives/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/shared/components/primitives/table";

export interface Column<T> {
  key: string;
  header: ReactNode;
  cell: (row: T) => ReactNode;
  /** Numbers and money align right. */
  align?: "left" | "right";
  className?: string;
}

export interface DataTableProps<T> {
  columns: Column<T>[];
  rows: T[];
  getRowId: (row: T) => string | number;
  loading?: boolean;
  /** Rows of skeleton while loading — match the page size. */
  skeletonRows?: number;
  /** Shown instead of the table body when there are no rows. */
  empty?: ReactNode;
  /** Makes the whole row open the record (Enter works too). */
  onRowClick?: (row: T) => void;
  /** Accessible name of a row, used when rows are clickable. */
  rowLabel?: (row: T) => string;
}

/** A plain data table: sticky header, density-aware rows, skeleton, empty state. */
export function DataTable<T>({
  columns,
  rows,
  getRowId,
  loading = false,
  skeletonRows = 5,
  empty,
  onRowClick,
  rowLabel,
}: DataTableProps<T>) {
  if (!loading && rows.length === 0 && empty) return <>{empty}</>;

  const onKeyDown = (row: T) => (e: KeyboardEvent<HTMLTableRowElement>) => {
    if (onRowClick && (e.key === "Enter" || e.key === " ")) {
      e.preventDefault();
      onRowClick(row);
    }
  };

  return (
    <Table>
      <TableHeader className="sticky top-0 z-10 bg-card">
        <TableRow>
          {columns.map((c) => (
            <TableHead key={c.key} className={cn(c.align === "right" && "text-right", c.className)}>
              {c.header}
            </TableHead>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {loading
          ? Array.from({ length: skeletonRows }, (_, i) => (
              <TableRow key={i}>
                {columns.map((c) => (
                  <TableCell key={c.key}>
                    <Skeleton className="h-4 w-full max-w-40" />
                  </TableCell>
                ))}
              </TableRow>
            ))
          : rows.map((row) => (
              <TableRow
                key={getRowId(row)}
                className={cn(onRowClick && "cursor-pointer")}
                onClick={onRowClick ? () => onRowClick(row) : undefined}
                onKeyDown={onRowClick ? onKeyDown(row) : undefined}
                tabIndex={onRowClick ? 0 : undefined}
                aria-label={onRowClick && rowLabel ? rowLabel(row) : undefined}
              >
                {columns.map((c) => (
                  <TableCell
                    key={c.key}
                    className={cn(c.align === "right" && "text-right tabular", c.className)}
                  >
                    {c.cell(row)}
                  </TableCell>
                ))}
              </TableRow>
            ))}
      </TableBody>
    </Table>
  );
}
