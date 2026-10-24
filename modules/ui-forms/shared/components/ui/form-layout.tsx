import type { ReactNode } from "react";
import { cn } from "cn";

/** A form section: title and description on the left, fields on the right (≥ 1024px). */
export function FormSection({ id, title, description, children, className }: { id?: string; title: string; description?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section id={id} className={cn("grid gap-x-10 gap-y-4 border-b border-border py-6 first:pt-0 last:border-b-0 lg:grid-cols-[240px_minmax(0,1fr)]", className)}>
      <div>
        <h2 className="text-lg font-semibold text-foreground">{title}</h2>
        {description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
      </div>
      <div className="min-w-0">{children}</div>
    </section>
  );
}

/** A grid of fields: one column on phones, `columns` from 640px. */
export function FieldGrid({ columns = 2, children, className }: { columns?: 1 | 2 | 3; children: ReactNode; className?: string }) {
  const cols = { 1: "", 2: "sm:grid-cols-2", 3: "sm:grid-cols-2 lg:grid-cols-3" }[columns];
  return <div className={cn("grid grid-cols-1 gap-4", cols, className)}>{children}</div>;
}

/** A form's action bar pinned to the bottom of the canvas: summary left, Batal · Simpan right. */
export function StickyFooter({ summary, children, className }: { summary?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        "sticky bottom-0 z-20 -mx-4 mt-6 flex flex-wrap items-center justify-between gap-3 border-t border-border bg-card/95 px-4 py-3 md:-mx-6 md:px-6",
        className,
      )}
    >
      <div className="min-w-0 text-sm text-muted-foreground">{summary}</div>
      <div className="flex flex-wrap items-center justify-end gap-2">{children}</div>
    </div>
  );
}

/** Buttons closing a form or a card, above a divider. */
export function ActionRow({ children, align = "end" }: { children: ReactNode; align?: "start" | "center" | "end" }) {
  const justify = { start: "justify-start", center: "justify-center", end: "justify-end" }[align];
  return <div className={cn("mt-6 flex flex-wrap items-center gap-2 border-t border-border pt-5", justify)}>{children}</div>;
}
