import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { KeyboardEvent, ReactNode, RefObject } from "react";
import { Plus, Trash2 } from "lucide-react";
import { cn } from "cn";
import { Button } from "@/shared/components/primitives/button";

export interface LineColumn<T> {
  id: string;
  header: string;
  /** CSS width, e.g. "9rem"; columns without one share the rest. */
  width?: string;
  align?: "left" | "right" | "center";
  /** A control in the header (e.g. "select all") instead of the text. */
  headerNode?: ReactNode;
  /** The cell — usually an input; index for labels ("Qty baris 3"). */
  render: (line: T, index: number) => ReactNode;
}

export interface LineItemsEditorProps<T> {
  lines: T[];
  getKey: (line: T, index: number) => string | number;
  columns: LineColumn<T>[];
  /** Adds an empty line; Enter in the last line calls it too. */
  onAdd?: () => void;
  onRemove?: (index: number) => void;
  addLabel?: string;
  emptyText?: string;
  /** Display only: no add or remove. */
  readOnly?: boolean;
  /** Under the table, e.g. totals. */
  footer?: ReactNode;
  rowClassName?: (line: T) => string | undefined;
  className?: string;
}

/** Under this container width (px) lines become stacked cards. */
const CARD_BREAKPOINT = 600;

function useNarrow(ref: RefObject<HTMLElement | null>, threshold: number): boolean {
  const [narrow, setNarrow] = useState(false);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = (w: number) => setNarrow(w > 0 && w < threshold);
    measure(el.getBoundingClientRect().width);
    if (typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver((entries) => measure(entries[0]?.contentRect.width ?? 0));
    ro.observe(el);
    return () => ro.disconnect();
  }, [ref, threshold]);
  return narrow;
}

const ALIGN = { left: "text-left", right: "text-right", center: "text-center" } as const;

/**
 * The editable grid of a document's lines (order items, journal lines). Tab moves between cells;
 * Enter in the last line adds one and focuses it. In a narrow container (phone, tablet with the
 * sidebar open) each line becomes a card with labels above its fields.
 */
export function LineItemsEditor<T>({
  lines,
  getKey,
  columns,
  onAdd,
  onRemove,
  addLabel = "Tambah baris",
  emptyText = "Belum ada baris.",
  readOnly = false,
  footer,
  rowClassName,
  className,
}: LineItemsEditorProps<T>) {
  const root = useRef<HTMLDivElement>(null);
  const focusLast = useRef(false);
  const narrow = useNarrow(root, CARD_BREAKPOINT);
  const canEdit = !readOnly;

  useEffect(() => {
    if (!focusLast.current) return;
    focusLast.current = false;
    const rows = root.current?.querySelectorAll("[data-line]");
    rows?.[rows.length - 1]
      ?.querySelector<HTMLElement>("input:not([type=hidden]):not(:disabled), select:not(:disabled), [aria-haspopup=listbox]")
      ?.focus();
  }, [lines.length]);

  function lineKeyDown(e: KeyboardEvent<HTMLElement>, index: number) {
    if (!canEdit || !onAdd || e.key !== "Enter" || index !== lines.length - 1) return;
    const el = e.target as HTMLElement;
    // Events from a portaled popover inside a cell bubble here through React: ignore them.
    if (!e.currentTarget.contains(el)) return;
    if (el.tagName !== "INPUT" || (el as HTMLInputElement).type === "checkbox") return;
    e.preventDefault();
    focusLast.current = true;
    onAdd();
  }

  const remove = (index: number) => (
    <Button variant="ghost" size="icon-sm" aria-label={`Hapus baris ${index + 1}`} onClick={() => onRemove?.(index)} className="text-muted-foreground hover:text-destructive">
      <Trash2 aria-hidden />
    </Button>
  );

  return (
    <div ref={root} className={cn("overflow-hidden rounded-lg border border-border", className)}>
      {narrow ? (
        lines.length === 0 ? (
          <p className="px-4 py-8 text-center text-sm text-muted-foreground">{emptyText}</p>
        ) : (
          <ol className="divide-y divide-border">
            {lines.map((line, index) => (
              <li key={getKey(line, index)} data-line="" className={cn("px-3 py-3", rowClassName?.(line))} onKeyDown={(e) => lineKeyDown(e, index)}>
                <div className="mb-2 flex items-center justify-between gap-2">
                  <span className="tabular text-xs font-semibold text-muted-foreground">Baris {index + 1}</span>
                  {canEdit && onRemove && remove(index)}
                </div>
                <div className="grid grid-cols-2 gap-x-3 gap-y-2">
                  {columns.map((c, ci) => (
                    <div key={c.id} className={cn("min-w-0", (ci === 0 || !c.width) && "col-span-2")}>
                      {c.header && <div className="mb-1 text-xs font-medium text-muted-foreground">{c.header}</div>}
                      <div className={cn("text-sm", c.align === "right" && "text-right")}>{c.render(line, index)}</div>
                    </div>
                  ))}
                </div>
              </li>
            ))}
          </ol>
        )
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[640px] border-collapse">
            <thead>
              <tr className="border-b border-border bg-background">
                <th scope="col" className="w-10 px-3 py-2 text-center text-xs font-semibold text-muted-foreground">
                  #
                </th>
                {columns.map((c) => (
                  <th
                    key={c.id}
                    scope="col"
                    style={c.width ? { width: c.width } : undefined}
                    className={cn("px-2 py-2 text-xs font-semibold whitespace-nowrap text-muted-foreground", ALIGN[c.align ?? "left"])}
                  >
                    {c.headerNode ?? c.header}
                  </th>
                ))}
                {canEdit && onRemove && (
                  <th scope="col" className="w-10">
                    <span className="sr-only">Hapus</span>
                  </th>
                )}
              </tr>
            </thead>
            <tbody>
              {lines.length === 0 ? (
                <tr>
                  <td colSpan={columns.length + (canEdit && onRemove ? 2 : 1)} className="px-4 py-8 text-center text-sm text-muted-foreground">
                    {emptyText}
                  </td>
                </tr>
              ) : (
                lines.map((line, index) => (
                  <tr
                    key={getKey(line, index)}
                    data-line=""
                    className={cn("border-b border-border align-top last:border-b-0", rowClassName?.(line))}
                    onKeyDown={(e) => lineKeyDown(e, index)}
                  >
                    <td className="tabular px-3 py-2.5 text-center text-xs text-muted-foreground">{index + 1}</td>
                    {columns.map((c) => (
                      <td key={c.id} className={cn("px-2 py-1.5 text-sm", ALIGN[c.align ?? "left"])}>
                        {c.render(line, index)}
                      </td>
                    ))}
                    {canEdit && onRemove && <td className="px-1 py-1.5 text-center">{remove(index)}</td>}
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}
      {(canEdit && onAdd) || footer ? (
        <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border bg-background px-3 py-2">
          {canEdit && onAdd ? (
            <Button variant="ghost" size="sm" onClick={onAdd}>
              <Plus aria-hidden /> {addLabel}
            </Button>
          ) : (
            <span />
          )}
          {canEdit && onAdd && lines.length > 0 && <span className="text-xs text-muted-foreground">Enter di baris terakhir menambah baris · Tab pindah sel</span>}
          {footer}
        </div>
      ) : null}
    </div>
  );
}
