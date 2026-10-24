import type { NavItem } from "@/shared/composition";

/** The app's name, taken from <title> (index.html) so the shell needs no configuration. */
export const appName = document.title || "App";

/** The entry whose path is the longest prefix of pathname — "/users/42" activates "/users". */
export function activeNavItem(items: NavItem[], pathname: string): NavItem | undefined {
  let best: NavItem | undefined;
  for (const item of items) {
    const matches = pathname === item.to || pathname.startsWith(item.to.endsWith("/") ? item.to : `${item.to}/`);
    if (matches && (!best || item.to.length > best.to.length)) best = item;
  }
  return best;
}
