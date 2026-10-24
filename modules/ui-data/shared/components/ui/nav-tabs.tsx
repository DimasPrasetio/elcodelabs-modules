import { useCallback } from "react";
import { NavLink, useSearchParams } from "react-router-dom";
import { cn } from "cn";

export interface NavTabItem {
  to: string;
  label: string;
  /** Match the path exactly (default true). */
  end?: boolean;
}

const TAB =
  "inline-flex items-center border-b-2 px-3 py-2 text-sm font-medium whitespace-nowrap transition-colors hover:no-underline";

/** Tabs that are real links — one URL per tab, so Back and the sidebar always agree. */
export function NavTabs({ items, label = "Bagian" }: { items: NavTabItem[]; label?: string }) {
  return (
    <div role="group" aria-label={label} className="flex gap-1 overflow-x-auto border-b border-border">
      {items.map((it) => (
        <NavLink
          key={it.to}
          to={it.to}
          end={it.end ?? true}
          className={({ isActive }) =>
            cn(TAB, isActive ? "border-primary text-foreground" : "border-transparent text-muted-foreground hover:text-foreground")
          }
        >
          {it.label}
        </NavLink>
      ))}
    </div>
  );
}

/** A tab kept in the query string (?tab=) for pages without a route per tab. */
export function useUrlTab<V extends string>(values: readonly V[], fallback: V, key = "tab"): [V, (v: V) => void] {
  const [params, setParams] = useSearchParams();
  const raw = params.get(key);
  const active = (values as readonly string[]).includes(raw ?? "") ? (raw as V) : fallback;
  const set = useCallback(
    (v: V) => {
      setParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          if (v === fallback) next.delete(key);
          else next.set(key, v);
          return next;
        },
        { replace: true },
      );
    },
    [fallback, key, setParams],
  );
  return [active, set];
}
