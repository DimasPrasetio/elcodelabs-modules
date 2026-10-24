import { useState } from "react";
import type { CSSProperties } from "react";
import { Check } from "lucide-react";
import { cn } from "cn";
import {
  SelectContent,
  SelectList,
  SelectMessage,
  SelectRoot,
  SelectSearch,
  SelectTrigger,
  optionSearchText,
} from "@/shared/components/ui/select-parts";
import type { SelectOption, SelectValue } from "@/shared/components/ui/select-parts";

export interface TreeOption extends SelectOption {
  /** 0 for roots. Options come in tree order (parent before children). */
  depth: number;
  /** Ancestor labels; the trigger shows "Gudang > Rak A > Baris 2". */
  path?: string[];
  /** A grouping node that cannot be chosen (shown with a "Grup" badge). */
  isGroup?: boolean;
}

export interface HierarchicalSelectProps {
  options: TreeOption[];
  value: SelectValue | null | undefined;
  onChange: (value: SelectValue | null, option?: TreeOption) => void;
  placeholder?: string;
  searchPlaceholder?: string;
  emptyText?: string;
  /** An extra first choice meaning "none", e.g. "Tanpa induk". */
  emptyOptionLabel?: string;
  disabled?: boolean;
  id?: string;
  name?: string;
  "aria-label"?: string;
  "aria-invalid"?: boolean;
  "aria-describedby"?: string;
  className?: string;
}

const GUIDE = 14; // px per level

/** A select over a small tree (locations, categories) drawn with guide lines. */
export function HierarchicalSelect({
  options,
  value,
  onChange,
  placeholder = "Pilih…",
  searchPlaceholder = "Cari…",
  emptyText = "Tidak ada pilihan",
  emptyOptionLabel,
  disabled,
  id,
  name,
  className,
  ...aria
}: HierarchicalSelectProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const q = query.trim().toLowerCase();
  const filtered = q ? options.filter((o) => optionSearchText(o).includes(q) || o.path?.join(" ").toLowerCase().includes(q)) : options;
  const selected = options.find((o) => o.value === value);
  const hasValue = value !== null && value !== undefined && value !== "";
  const label = selected ? (selected.path?.length ? [...selected.path, selected.label].join(" > ") : selected.label) : (emptyOptionLabel ?? placeholder);

  function choose(o: TreeOption | null) {
    if (o?.disabled || o?.isGroup) return;
    onChange(o ? o.value : null, o ?? undefined);
    setOpen(false);
  }

  return (
    <SelectRoot
      open={open && !disabled}
      onOpenChange={(next) => {
        if (next) setQuery("");
        setOpen(next);
      }}
    >
      {name && <input type="hidden" name={name} value={hasValue ? String(value) : ""} />}
      <SelectTrigger id={id} disabled={disabled} label={label} empty={!selected} open={open} className={className} {...aria} />
      <SelectContent className="w-auto min-w-[max(var(--radix-popover-trigger-width),min(360px,calc(100vw-24px)))]">
        <SelectSearch value={query} onChange={(e) => setQuery(e.target.value)} placeholder={searchPlaceholder} aria-label={searchPlaceholder} />
        <SelectList className="gap-0.5">
          {emptyOptionLabel && !q && (
            <button
              type="button"
              role="option"
              aria-selected={!hasValue}
              data-select-option=""
              onClick={() => choose(null)}
              className={cn(
                "flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm text-muted-foreground italic hover:bg-background",
                !hasValue && "bg-primary/10 font-medium text-primary not-italic",
              )}
            >
              {emptyOptionLabel}
            </button>
          )}
          {filtered.length === 0 ? (
            <SelectMessage>{emptyText}</SelectMessage>
          ) : (
            filtered.map((o) => {
              const active = o.value === value;
              const depth = q ? 0 : o.depth;
              return (
                <button
                  key={o.value}
                  type="button"
                  role="option"
                  aria-selected={active}
                  data-select-option=""
                  disabled={o.disabled || o.isGroup}
                  onClick={() => choose(o)}
                  style={{ "--guide": `${depth * GUIDE}px` } as CSSProperties}
                  className={cn(
                    "group relative flex min-h-10 w-full items-start justify-between gap-2 rounded-md py-2 pr-3 pl-[calc(0.625rem+var(--guide))] text-left text-sm text-foreground transition-colors enabled:hover:bg-background focus-visible:bg-background focus-visible:outline-none disabled:cursor-default",
                    active && "bg-primary/10 font-medium text-primary",
                  )}
                >
                  {depth > 0 && (
                    <span
                      aria-hidden
                      className={cn(
                        "absolute top-0 bottom-1/2 w-3 rounded-bl border-b border-l border-border",
                        active ? "border-primary" : "group-hover:border-primary",
                      )}
                      style={{ left: `calc(${(depth - 1) * GUIDE}px + 0.875rem)` }}
                    />
                  )}
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="flex flex-wrap items-center gap-1.5">
                      <span className={cn("[overflow-wrap:break-word]", o.isGroup && "font-semibold text-muted-foreground")}>{o.label}</span>
                      {o.isGroup && (
                        <span className="rounded-full border border-border bg-muted px-1.5 py-px text-2xs font-semibold text-muted-foreground uppercase">Grup</span>
                      )}
                    </span>
                    {q && o.path?.length ? <span className="text-xs font-normal text-muted-foreground">{o.path.join(" > ")}</span> : null}
                    {o.description ? <span className="text-xs font-normal text-muted-foreground">{o.description}</span> : null}
                  </span>
                  {active && <Check size={16} aria-hidden className="mt-0.5 ml-1 shrink-0" />}
                </button>
              );
            })
          )}
        </SelectList>
      </SelectContent>
    </SelectRoot>
  );
}
