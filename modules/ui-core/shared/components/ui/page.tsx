import type { ReactNode } from "react";
import { cn } from "cn";

/** A page's root: a vertical stack with 24px between sections — no ad-hoc margins. */
export function Page({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn("flex flex-col gap-6", className)}>{children}</div>;
}

/** Two columns for a record page: content (left) and summary/metadata (right, sticky). */
export function PageColumns({ main, aside }: { main: ReactNode; aside: ReactNode }) {
  return (
    <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
      <div className="flex min-w-0 flex-col gap-6">{main}</div>
      <aside className="flex min-w-0 flex-col gap-6 xl:sticky xl:top-20">{aside}</aside>
    </div>
  );
}
