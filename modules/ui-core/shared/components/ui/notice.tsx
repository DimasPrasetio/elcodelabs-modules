import type { ReactNode } from "react";
import { AlertTriangle, CheckCircle2, Info, XCircle } from "lucide-react";
import { cn } from "cn";
import type { Tone } from "@/shared/lib/tone";

const WRAPPER: Record<Tone, string> = {
  success: "bg-success-soft border-success/25",
  warning: "bg-warning-soft border-warning/25",
  danger: "bg-destructive-soft border-destructive/25",
  info: "bg-accent border-primary/15",
  neutral: "bg-muted border-border",
  muted: "bg-muted border-border",
};

const TITLE: Record<Tone, string> = {
  success: "text-success",
  warning: "text-warning",
  danger: "text-destructive",
  info: "text-accent-foreground",
  neutral: "text-foreground",
  muted: "text-muted-foreground",
};

const ICON = { success: CheckCircle2, warning: AlertTriangle, danger: XCircle, info: Info, neutral: Info, muted: Info } as const;

/** An inline message box: tone, title, and optional details. Danger is announced (role=alert). */
export function Notice({ tone, title, children, className }: { tone: Tone; title: string; children?: ReactNode; className?: string }) {
  const Icon = ICON[tone];
  return (
    <div role={tone === "danger" ? "alert" : undefined} className={cn("flex gap-3 rounded-lg border p-3.5", WRAPPER[tone], className)}>
      <Icon size={18} aria-hidden className={cn("mt-px flex-none", TITLE[tone])} />
      <div className="min-w-0">
        <p className={cn("text-sm font-semibold", TITLE[tone])}>{title}</p>
        {children && <div className="mt-0.5 text-sm text-foreground">{children}</div>}
      </div>
    </div>
  );
}
