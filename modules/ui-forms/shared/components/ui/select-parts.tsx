import { forwardRef } from "react";
import type { ComponentProps, KeyboardEvent, ReactNode } from "react";
import { Check, ChevronDown, X } from "lucide-react";
import { Popover } from "radix-ui";
import { cn } from "cn";
import { FIELD } from "@/shared/lib/field-classes";

// The parts every searchable select is built from (SearchSelect, AsyncSearchSelect,
// HierarchicalSelect): one trigger look, one popover, one keyboard model. Built on Radix
// Popover, so a select also works inside a Dialog or a Sheet.

export type SelectValue = string | number;

export interface SelectOption {
  value: SelectValue;
  label: string;
  /** A second line under the label. */
  description?: ReactNode;
  disabled?: boolean;
  /** Extra words the search matches, e.g. a code. */
  searchText?: string;
}

export const SelectRoot = Popover.Root;

/** The list height follows the room on screen, never below 120px. */
export const LIST_MAX_HEIGHT = "max(120px, min(20rem, calc(var(--radix-popover-content-available-height) - 56px)))";

export interface SelectTriggerProps extends Omit<ComponentProps<"button">, "children"> {
  label: ReactNode;
  /** Shows the label muted (nothing chosen). */
  empty: boolean;
  open: boolean;
  onClear?: () => void;
}

/** The closed select: looks like every other field, chevron turns when open. */
export const SelectTrigger = forwardRef<HTMLButtonElement, SelectTriggerProps>(function SelectTrigger(
  { label, empty, open, onClear, className, ...props },
  ref,
) {
  return (
    <Popover.Trigger asChild>
      <button
        ref={ref}
        type="button"
        aria-haspopup="listbox"
        aria-expanded={open}
        className={cn(FIELD, "flex cursor-pointer items-center justify-between gap-2 text-left disabled:opacity-70", className)}
        {...props}
      >
        <span className={cn("min-w-0 flex-1 truncate", empty ? "text-muted-foreground" : "text-foreground")}>{label}</span>
        <span className="flex shrink-0 items-center gap-1">
          {onClear && (
            <span
              role="button"
              tabIndex={0}
              aria-label="Hapus pilihan"
              className="rounded p-0.5 text-muted-foreground hover:text-foreground"
              onClick={(e) => {
                e.stopPropagation();
                onClear();
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  e.stopPropagation();
                  onClear();
                }
              }}
            >
              <X size={14} aria-hidden />
            </span>
          )}
          <ChevronDown size={16} aria-hidden className={cn("text-muted-foreground transition-transform", open && "rotate-180")} />
        </span>
      </button>
    </Popover.Trigger>
  );
});

/**
 * Keyboard inside the popover: ↓/↑ move between the search box and the options (wrapping), ↑
 * from the first option returns to the search box, Enter in the search box picks the first
 * enabled option. Options carry data-select-option and are chosen by click, so each select
 * keeps its own choose handler.
 */
function listKeyDown(e: KeyboardEvent<HTMLElement>) {
  if (e.defaultPrevented) {
    if (e.key === "Enter") e.stopPropagation();
    return;
  }
  const root = e.currentTarget;
  const options = Array.from(root.querySelectorAll<HTMLElement>("[data-select-option]:not(:disabled)"));
  const input = root.querySelector<HTMLInputElement>("input");
  const target = e.target as HTMLElement;
  const idx = options.indexOf(target);
  // The popover is portaled, but React events still bubble to parents in the component tree
  // (e.g. a line editor adding a row on Enter): stop them once handled here.
  if (e.key === "ArrowDown") {
    if (options.length === 0) return;
    e.preventDefault();
    e.stopPropagation();
    (options[idx + 1] ?? options[0]).focus();
  } else if (e.key === "ArrowUp") {
    if (idx < 0) return;
    e.preventDefault();
    e.stopPropagation();
    if (idx === 0) input?.focus();
    else options[idx - 1].focus();
  } else if (e.key === "Enter" && target === input) {
    e.preventDefault();
    e.stopPropagation();
    options[0]?.click();
  }
}

/** The open panel, as wide as the trigger (or wider, for trees). */
export function SelectContent({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <Popover.Portal>
      <Popover.Content
        align="start"
        sideOffset={4}
        collisionPadding={8}
        onKeyDown={listKeyDown}
        className={cn(
          "z-[60] flex w-(--radix-popover-trigger-width) max-w-[calc(100vw-24px)] min-w-0 flex-col overflow-hidden rounded-xl border border-border bg-card shadow-(--shadow-pop) data-[state=open]:animate-in data-[state=open]:fade-in-0",
          className,
        )}
      >
        {children}
      </Popover.Content>
    </Popover.Portal>
  );
}

/** The search box at the top of the panel. */
export function SelectSearch(props: ComponentProps<"input">) {
  return (
    <div className="border-b border-border p-2">
      <input autoFocus {...props} className={cn(FIELD, "h-8 px-2.5 text-sm shadow-none", props.className)} />
    </div>
  );
}

export function SelectList({ children, busy, className }: { children: ReactNode; busy?: boolean; className?: string }) {
  return (
    <div role="listbox" aria-busy={busy || undefined} className={cn("flex flex-col overflow-y-auto p-1", className)} style={{ maxHeight: LIST_MAX_HEIGHT }}>
      {children}
    </div>
  );
}

/** One option. Active = the chosen one: navy tint and a check. */
export function SelectOptionButton({
  option,
  active,
  onChoose,
  className,
  children,
}: {
  option: SelectOption;
  active: boolean;
  onChoose: () => void;
  className?: string;
  children?: ReactNode;
}) {
  return (
    <button
      type="button"
      role="option"
      aria-selected={active}
      data-select-option=""
      disabled={option.disabled}
      onClick={onChoose}
      className={cn(
        "flex w-full items-center justify-between gap-2 rounded-md px-3 py-2 text-left text-sm text-foreground transition-colors enabled:hover:bg-background focus-visible:bg-background focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-55",
        active && "bg-primary/10 font-medium text-primary enabled:hover:bg-primary/10",
        className,
      )}
    >
      {children ?? (
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="[overflow-wrap:break-word]">{option.label}</span>
          {option.description ? <span className="text-xs font-normal text-muted-foreground">{option.description}</span> : null}
        </span>
      )}
      {active && <Check size={16} aria-hidden className="ml-1 shrink-0" />}
    </button>
  );
}

/** A line in the list that is not an option: empty, loading, an error. */
export function SelectMessage({ children, role }: { children: ReactNode; role?: string }) {
  return (
    <div role={role} className="p-3 text-center text-sm text-muted-foreground">
      {children}
    </div>
  );
}

/** Lower-cased text a query is matched against. */
export function optionSearchText(o: SelectOption): string {
  const desc = typeof o.description === "string" ? o.description : "";
  return [String(o.value), o.label, desc, o.searchText].filter(Boolean).join(" ").toLowerCase();
}
