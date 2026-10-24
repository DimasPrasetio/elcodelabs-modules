import { Check } from "lucide-react";
import { cn } from "cn";

export interface StepperStep {
  key: string;
  label: string;
}

/**
 * Horizontal steps of a flow or a document's lifecycle: past = check, current = navy, next = gray.
 * stopped marks a flow halted at the current step (e.g. cancelled): a dashed gray line.
 */
export function Stepper({ steps, current, stopped = false, label = "Tahapan", className }: { steps: StepperStep[]; current: string; stopped?: boolean; label?: string; className?: string }) {
  const idx = steps.findIndex((s) => s.key === current);
  return (
    <ol className={cn("flex w-full items-start", className)} aria-label={label}>
      {steps.map((s, i) => {
        const done = idx >= 0 && i < idx;
        const active = i === idx;
        return (
          <li
            key={s.key}
            aria-current={active ? "step" : undefined}
            className={cn(
              "relative flex min-w-0 flex-1 flex-col items-center gap-1.5 text-center",
              i === 0 && "items-start text-left",
              i === steps.length - 1 && "items-end text-right",
            )}
          >
            {i > 0 && (
              <span
                aria-hidden
                className={cn(
                  "absolute top-3 right-1/2 left-[calc(-50%+0.75rem)] h-0.5",
                  done || active ? (stopped && active ? "border-t-2 border-dashed border-input" : "bg-primary") : "bg-border",
                )}
              />
            )}
            <span
              className={cn(
                "relative z-[1] flex size-6 items-center justify-center rounded-full border-2 text-2xs font-semibold",
                done && "border-primary bg-primary text-primary-foreground",
                active && !stopped && "border-primary bg-card text-primary",
                active && stopped && "border-dashed border-input bg-card text-muted-foreground",
                !done && !active && "border-border bg-card text-muted-foreground",
              )}
            >
              {done ? <Check size={12} aria-hidden /> : i + 1}
            </span>
            <span className={cn("max-w-full truncate text-xs", active ? "font-semibold text-foreground" : "text-muted-foreground")}>{s.label}</span>
          </li>
        );
      })}
    </ol>
  );
}
