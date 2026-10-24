import type { ReactNode } from "react";

/** A card of filter fields, aligned on their baseline — for report pages with many filters. */
export function FilterBar({ children }: { children: ReactNode }) {
  return <div className="flex flex-wrap items-end gap-x-4 gap-y-3 rounded-xl border border-border bg-card px-5 py-4">{children}</div>;
}
