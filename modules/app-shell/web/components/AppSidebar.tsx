import { Link, useLocation } from "react-router-dom";
import { useAccess } from "@/shared/access";
import { useNavItems, useSlot } from "@/shared/composition";
import type { NavItem } from "@/shared/composition";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSkeleton,
  SidebarRail,
} from "@/shared/components/primitives/sidebar";
import { activeNavItem, appName } from "@/modules/app-shell/lib/nav";

// Bingkai Navy: quiet items, the active one a pill with its icon in the accent color.
const itemClass =
  "text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground " +
  "data-[active=true]:bg-sidebar-active data-[active=true]:text-sidebar-strong " +
  "data-[active=true]:[&>svg]:text-sidebar-primary";

function groupItems(items: NavItem[]): [string, NavItem[]][] {
  const groups = new Map<string, NavItem[]>();
  for (const item of items) {
    const key = item.group ?? "";
    groups.set(key, [...(groups.get(key) ?? []), item]);
  }
  return [...groups.entries()];
}

export function AppSidebar() {
  const { can, ready } = useAccess();
  // Entries the user may not open are not shown (the API refuses them anyway).
  const items = useNavItems().filter((item) => !item.permission || can(item.permission));
  const footer = useSlot("sidebar-footer");
  const { pathname } = useLocation();
  const active = activeNavItem(items, pathname);

  return (
    <Sidebar variant="inset" collapsible="icon">
      <SidebarHeader>
        <div className="flex h-10 items-center gap-2 px-2">
          <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-sidebar-primary font-display text-sm font-semibold text-sidebar-primary-foreground">
            {appName.charAt(0).toUpperCase()}
          </div>
          <span className="truncate font-display text-sm font-semibold text-sidebar-strong group-data-[collapsible=icon]:hidden">
            {appName}
          </span>
        </div>
      </SidebarHeader>
      <SidebarContent>
        {/* Until permissions arrive, placeholders instead of a menu that pops in or flashes empty. */}
        {!ready && (
          <SidebarGroup>
            <SidebarMenu>
              {[0, 1, 2, 3].map((i) => (
                <SidebarMenuItem key={i}>
                  <SidebarMenuSkeleton showIcon />
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroup>
        )}
        {ready && groupItems(items).map(([group, groupItems]) => (
          <SidebarGroup key={group || "_"}>
            {group && (
              <SidebarGroupLabel className="text-2xs tracking-wide text-sidebar-muted uppercase">
                {group}
              </SidebarGroupLabel>
            )}
            <SidebarGroupContent>
              <SidebarMenu>
                {groupItems.map((item) => (
                  <SidebarMenuItem key={item.to}>
                    <SidebarMenuButton
                      asChild
                      isActive={item === active}
                      tooltip={item.label}
                      className={itemClass}
                    >
                      <Link to={item.to}>
                        {item.icon && <item.icon />}
                        <span>{item.label}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        ))}
      </SidebarContent>
      {footer.length > 0 && <SidebarFooter>{footer}</SidebarFooter>}
      <SidebarRail />
    </Sidebar>
  );
}
