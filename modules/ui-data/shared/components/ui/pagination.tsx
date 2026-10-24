import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";

export interface PaginationProps {
  page: number;
  limit: number;
  total: number;
  onPageChange: (page: number) => void;
  /** Lets the user choose rows per page. */
  onLimitChange?: (limit: number) => void;
  limitOptions?: readonly number[];
  /** What is being counted: "1–25 dari 128 data". */
  noun?: string;
}

/**
 * "1–25 dari 128 data", rows per page, and previous/next — pagination, never infinite scroll.
 * Takes the API's meta as is: <Pagination {...meta} onPageChange={…} />.
 */
export function Pagination({ page, limit, total, onPageChange, onLimitChange, limitOptions = [10, 25, 50, 100], noun = "data" }: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(total / limit));
  const from = total === 0 ? 0 : (page - 1) * limit + 1;
  const to = Math.min(page * limit, total);
  const options = limitOptions.includes(limit) ? limitOptions : [...limitOptions, limit].sort((a, b) => a - b);

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border px-5 py-3">
      <p className="tabular text-sm text-muted-foreground">
        <span className="font-medium text-foreground">
          {from}–{to}
        </span>{" "}
        dari <span className="font-medium text-foreground">{total}</span> {noun}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        {onLimitChange && (
          <label className="mr-2 flex items-center gap-2 text-sm text-muted-foreground">
            Baris
            <select
              value={limit}
              onChange={(e) => onLimitChange(Number(e.target.value))}
              className="h-8 rounded-md border border-input bg-card px-2 text-sm text-foreground"
            >
              {options.map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </select>
          </label>
        )}
        <span className="tabular mr-1 text-sm text-muted-foreground">
          Hal. {page} / {totalPages}
        </span>
        <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
          <ChevronLeft aria-hidden /> Sebelumnya
        </Button>
        <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => onPageChange(page + 1)}>
          Berikutnya <ChevronRight aria-hidden />
        </Button>
      </div>
    </div>
  );
}
