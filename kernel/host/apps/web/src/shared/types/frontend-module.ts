import type { ComponentType, ReactNode } from "react";
import type { RouteObject } from "react-router-dom";
import type { NavItem, SlotName } from "@/shared/composition";

/**
 * What a frontend module exports from its module.tsx. Modules never import each other: they
 * contribute routes, navigation, and slot content here, and app/ composes them.
 */
export interface FrontendModule {
  id: string;
  /** Reachable without signing in. Pages must be lazy-loaded. */
  public?: RouteObject[];
  /** Behind the guard (when a module provides one), inside the layout (likewise). */
  protected?: RouteObject[];
  /** Entries for the app's navigation, rendered by whichever module provides the layout. */
  nav?: NavItem[];
  /** Content for named places in the layout, e.g. the user menu in "sidebar-footer". */
  slots?: Partial<Record<SlotName, ReactNode>>;
  /** Wraps all protected routes; renders <Outlet /> when access is allowed. At most one module. */
  guard?: ReactNode;
  /** The frame around protected pages; renders <Outlet />. At most one module. */
  layout?: ReactNode;
  /** Wraps the whole app, e.g. to provide AccessContext. Providers nest in registry order. */
  provider?: ComponentType<{ children: ReactNode }>;
}
