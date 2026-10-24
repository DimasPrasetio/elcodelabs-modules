import { cn } from "cn";
import { Skeleton } from "@/shared/components/primitives/skeleton";

// Loading placeholders shaped like the result, announced to screen readers.

export function TableSkeleton({ rows = 8, cols = 5, className }: { rows?: number; cols?: number; className?: string }) {
  return (
    <div role="status" aria-busy="true" aria-label="Memuat data" className={cn("p-4", className)}>
      <div className="mb-3 flex gap-4">
        {Array.from({ length: cols }, (_, i) => (
          <Skeleton key={i} className="h-3 flex-1" />
        ))}
      </div>
      <div className="space-y-3">
        {Array.from({ length: rows }, (_, r) => (
          <div key={r} className="flex gap-4">
            {Array.from({ length: cols }, (_, c) => (
              <Skeleton key={c} className="h-5 flex-1" />
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

export function CardSkeleton({ className }: { className?: string }) {
  return (
    <div role="status" aria-busy="true" aria-label="Memuat" className={cn("rounded-xl border border-border bg-card p-4", className)}>
      <Skeleton className="h-3 w-24" />
      <Skeleton className="mt-3 h-8 w-36" />
      <Skeleton className="mt-2 h-3 w-20" />
    </div>
  );
}

export function DetailSkeleton() {
  return (
    <div role="status" aria-busy="true" aria-label="Memuat halaman" className="space-y-5">
      <div className="space-y-2">
        <Skeleton className="h-7 w-64" />
        <Skeleton className="h-4 w-96 max-w-full" />
      </div>
      <div className="grid gap-5 xl:grid-cols-[2fr_1fr]">
        <div className="rounded-xl border border-border bg-card">
          <TableSkeleton rows={5} cols={4} />
        </div>
        <div className="space-y-3 rounded-xl border border-border bg-card p-5">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-5/6" />
          <Skeleton className="h-6 w-40" />
        </div>
      </div>
    </div>
  );
}
