import type { ReactNode } from "react";
import { cn } from "cn";

export interface SectionCardProps {
  title?: string;
  description?: ReactNode;
  actions?: ReactNode;
  /** "plain" drops the card frame (a section inside a page). */
  variant?: "card" | "plain";
  /** No body padding — for a table that manages its own space. */
  flush?: boolean;
  className?: string;
  children: ReactNode;
}

/** A titled section: border-only card, header row with actions, 20px body. */
export function SectionCard({ title, description, actions, variant = "card", flush = false, className, children }: SectionCardProps) {
  const card = variant === "card";
  return (
    <section className={cn(card && "overflow-hidden rounded-xl border border-border bg-card", className)}>
      {(title || actions) && (
        <div className={cn("flex flex-wrap items-center justify-between gap-3", card ? "border-b border-border px-5 py-4" : "pb-3")}>
          <div className="min-w-0">
            {title && <h2 className="text-lg font-semibold text-foreground">{title}</h2>}
            {description && <p className="mt-0.5 text-sm text-muted-foreground">{description}</p>}
          </div>
          {actions && <div className="flex items-center gap-2">{actions}</div>}
        </div>
      )}
      <div className={card && !flush ? "p-5" : undefined}>{children}</div>
    </section>
  );
}
