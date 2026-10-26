import { createContext, useContext } from "react";
import type { ComponentType, ReactNode } from "react";

/** One navigation entry a module contributes. */
export interface NavItem {
  label: string;
  to: string;
  icon?: ComponentType<{ className?: string }>;
  /** Group heading; entries without one form the first, unlabeled group. */
  group?: string;
  /** Lower comes first within its group. */
  order?: number;
  /** Shown only to users who hold it (see useCan). The backend still enforces it. */
  permission?: string;
}

/** Named places in the layout that modules can fill. */
/**
 * Where modules put content into another module's screen. Open by design: a module hosting a
 * slot names it "<owner>:<place>" (e.g. "user-menu:items", "settings:sections") and documents
 * it; the frame's own two are "sidebar-footer" and "header-end".
 */
export type SlotName = "sidebar-footer" | "header-end" | (string & {});

export interface Composition {
  nav: NavItem[];
  slots: Partial<Record<SlotName, ReactNode[]>>;
}

const CompositionContext = createContext<Composition>({ nav: [], slots: {} });

/** Provided once by app/ from the registered modules. */
export const CompositionProvider = CompositionContext.Provider;

/** Navigation entries from every module, sorted by order. */
export function useNavItems(): NavItem[] {
  return useContext(CompositionContext).nav;
}

/** Everything modules put into the named slot, in registration order. */
export function useSlot(name: SlotName): ReactNode[] {
  return useContext(CompositionContext).slots[name] ?? [];
}
