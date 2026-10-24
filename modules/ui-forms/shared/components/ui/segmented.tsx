import { cn } from "cn";

export interface SegmentedOption<V extends string = string> {
  value: V;
  label: string;
  /** A count badge, e.g. how many records have this status. */
  count?: number;
}

export interface SegmentedProps<V extends string = string> {
  options: SegmentedOption<V>[];
  value: V;
  onChange: (value: V) => void;
  "aria-label"?: string;
}

/** One choice out of a few, as a pill switch — views, status filters, periods. */
export function Segmented<V extends string = string>({ options, value, onChange, "aria-label": ariaLabel }: SegmentedProps<V>) {
  return (
    <div role="group" aria-label={ariaLabel} className="inline-flex flex-wrap gap-0.5 rounded-lg bg-muted p-[3px]">
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            aria-pressed={active}
            onClick={() => onChange(o.value)}
            className={cn(
              "cursor-pointer rounded-md px-3 py-1 text-sm font-medium transition-[background-color,color,box-shadow] duration-150",
              active ? "bg-card text-foreground shadow-(--shadow-md)" : "text-muted-foreground hover:text-foreground",
            )}
          >
            {o.label}
            {o.count !== undefined && (
              <span className={cn("tabular ml-1.5 rounded-full px-1.5 text-2xs", active ? "bg-accent text-primary" : "bg-card text-muted-foreground")}>
                {o.count}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
