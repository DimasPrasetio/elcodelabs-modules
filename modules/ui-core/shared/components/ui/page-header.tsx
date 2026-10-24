import type { ReactNode } from "react";

export interface PageHeaderProps {
  title: string;
  description?: ReactNode;
  /** A badge or status beside the title. */
  meta?: ReactNode;
  /** A small line under the title, e.g. "Toko Maju · 16 Okt 2026". */
  subtitle?: ReactNode;
  /** Page actions; the main action goes last (rightmost). At most one primary button. */
  actions?: ReactNode;
  /** A band under the header, e.g. tabs or a stepper. */
  children?: ReactNode;
}

/** The page's one <h1> — the shell's breadcrumb never is one. */
export function PageHeader({ title, description, meta, subtitle, actions, children }: PageHeaderProps) {
  return (
    <div className="page-header">
      <div className="flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <h1 className="font-display text-2xl leading-tight font-semibold tracking-[-0.02em] text-foreground">{title}</h1>
            {meta}
          </div>
          {subtitle && <div className="mt-1 text-sm text-muted-foreground">{subtitle}</div>}
          {description && <p className="mt-1 max-w-[68ch] text-sm text-muted-foreground">{description}</p>}
        </div>
        {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
      </div>
      {children && <div className="mt-4">{children}</div>}
    </div>
  );
}
