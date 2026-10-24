import type { ComponentType, ReactNode } from "react";
import { Inbox } from "lucide-react";

export interface EmptyStateProps {
  title: string;
  description?: ReactNode;
  icon?: ComponentType<{ className?: string; size?: number }>;
  /** The next step, e.g. "Buat produk pertama". */
  action?: ReactNode;
}

/** Nothing to show — always an invitation to the next action. */
export function EmptyState({ title, description, icon: Icon = Inbox, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center px-6 py-14 text-center">
      <span className="mb-3 flex size-11 items-center justify-center rounded-full bg-muted text-muted-foreground">
        <Icon size={20} aria-hidden />
      </span>
      <p className="text-base font-semibold text-foreground">{title}</p>
      {description && <p className="mx-auto mt-1 max-w-[420px] text-sm text-muted-foreground">{description}</p>}
      {action && <div className="mt-4 flex justify-center">{action}</div>}
    </div>
  );
}
