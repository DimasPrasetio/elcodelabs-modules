import type { ReactNode } from "react";

export interface PageHeaderProps {
  title: string;
  description?: ReactNode;
  /** A badge or status beside the title. */
  meta?: ReactNode;
  /** Page actions; the main action goes last (rightmost). At most one primary button. */
  actions?: ReactNode;
}

/** The page's one <h1>, its description, and its actions. */
export function PageHeader({ title, description, meta, actions }: PageHeaderProps) {
  return (
    <div className="mb-6 flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <h1 className="font-display text-2xl leading-tight font-semibold tracking-tight">{title}</h1>
          {meta}
        </div>
        {description && <p className="mt-1 max-w-prose text-sm text-muted-foreground">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}
