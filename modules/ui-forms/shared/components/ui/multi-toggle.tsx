import { cn } from "cn";

export interface MultiToggleOption<V extends string = string> {
  value: V;
  label: string;
}

export interface MultiToggleProps<V extends string = string> {
  options: readonly MultiToggleOption<V>[];
  /** Chosen values; empty means "all". */
  value: readonly V[];
  onChange: (value: V[]) => void;
  "aria-label"?: string;
}

/** Several choices as chips — e.g. a multi-status filter. */
export function MultiToggle<V extends string = string>({ options, value, onChange, "aria-label": ariaLabel }: MultiToggleProps<V>) {
  function toggle(v: V) {
    onChange(value.includes(v) ? value.filter((x) => x !== v) : [...value, v]);
  }
  return (
    <div role="group" aria-label={ariaLabel} className="flex flex-wrap gap-1.5">
      {options.map((o) => {
        const active = value.includes(o.value);
        return (
          <button
            key={o.value}
            type="button"
            aria-pressed={active}
            onClick={() => toggle(o.value)}
            className={cn(
              "cursor-pointer rounded-full border px-3 py-1.5 text-sm font-medium transition-colors",
              active ? "border-primary bg-primary text-primary-foreground" : "border-border bg-card text-muted-foreground hover:text-foreground",
            )}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
