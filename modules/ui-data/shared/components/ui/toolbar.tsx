import { useState } from "react";
import type { ReactNode } from "react";
import { ListFilter, X } from "lucide-react";
import { cn } from "cn";
import { Button } from "@/shared/components/primitives/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/shared/components/primitives/popover";

export interface ActiveFilter {
  key: string;
  label: string;
  onRemove: () => void;
}

export interface ToolbarProps {
  search?: ReactNode;
  /** Up to three primary filters, always visible. */
  primary?: ReactNode;
  /** More filters, in a "Filter" popover. */
  more?: ReactNode;
  moreActiveCount?: number;
  /** Active filters as removable chips. */
  active?: ActiveFilter[];
  onClearAll?: () => void;
  /** Toolbar actions on the right (columns, export). */
  right?: ReactNode;
  /** No card frame — when the toolbar sits inside a card already. */
  plain?: boolean;
  className?: string;
}

/** A list's toolbar: search, a few filters, more in a popover, active filters as chips. */
export function Toolbar({ search, primary, more, moreActiveCount = 0, active = [], onClearAll, right, plain = false, className }: ToolbarProps) {
  const [open, setOpen] = useState(false);
  return (
    <div className={cn(plain ? "border-b border-border px-5 py-3" : "rounded-xl border border-border bg-card px-4 py-3", className)}>
      <div className="flex flex-wrap items-end gap-3">
        {search && <div className="min-w-[220px] flex-1 sm:max-w-sm">{search}</div>}
        {primary}
        {more && (
          <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
              <Button variant="outline">
                <ListFilter aria-hidden /> Filter
                {moreActiveCount > 0 && (
                  <span className="tabular rounded-full bg-primary px-1.5 text-2xs font-semibold text-primary-foreground">{moreActiveCount}</span>
                )}
              </Button>
            </PopoverTrigger>
            <PopoverContent align="start" className="flex w-80 flex-col gap-3 p-4">
              {more}
            </PopoverContent>
          </Popover>
        )}
        {right && <div className="ml-auto flex max-w-full flex-wrap items-center justify-end gap-2">{right}</div>}
      </div>
      {active.length > 0 && (
        <div className="mt-3 flex flex-wrap items-center gap-2 border-t border-border pt-3">
          {active.map((f) => (
            <span key={f.key} className="inline-flex items-center gap-1 rounded-full bg-accent py-0.5 pr-1 pl-2.5 text-xs font-medium text-accent-foreground">
              {f.label}
              <button type="button" onClick={f.onRemove} aria-label={`Hapus filter ${f.label}`} className="rounded-full p-0.5 hover:bg-primary/10">
                <X size={12} aria-hidden />
              </button>
            </span>
          ))}
          {onClearAll && (
            <button type="button" onClick={onClearAll} className="text-xs font-medium text-primary hover:underline">
              Hapus semua
            </button>
          )}
        </div>
      )}
    </div>
  );
}
