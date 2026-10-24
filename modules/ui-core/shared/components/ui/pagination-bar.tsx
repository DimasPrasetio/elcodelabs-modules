import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import type { PageMeta } from "@/shared/types/api";

export interface PaginationBarProps {
  meta: PageMeta;
  onPageChange: (page: number) => void;
}

/** "1–25 of 128" with previous/next — pagination, never infinite scroll. */
export function PaginationBar({ meta, onPageChange }: PaginationBarProps) {
  const from = meta.total === 0 ? 0 : (meta.page - 1) * meta.limit + 1;
  const to = Math.min(meta.page * meta.limit, meta.total);
  return (
    <div className="flex items-center justify-between gap-4 border-t px-4 py-3 text-sm text-muted-foreground">
      <span className="tabular">
        {from}–{to} of {meta.total}
      </span>
      <div className="flex items-center gap-1">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Previous page"
          disabled={meta.page <= 1}
          onClick={() => onPageChange(meta.page - 1)}
        >
          <ChevronLeft />
        </Button>
        <span className="tabular px-2">
          {meta.page} / {Math.max(meta.total_pages, 1)}
        </span>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Next page"
          disabled={meta.page >= meta.total_pages}
          onClick={() => onPageChange(meta.page + 1)}
        >
          <ChevronRight />
        </Button>
      </div>
    </div>
  );
}
