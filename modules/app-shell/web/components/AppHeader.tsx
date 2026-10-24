import { Fragment } from "react";
import { useLocation } from "react-router-dom";
import { useNavItems, useSlot } from "@/shared/composition";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/shared/components/primitives/breadcrumb";
import { Separator } from "@/shared/components/primitives/separator";
import { SidebarTrigger } from "@/shared/components/primitives/sidebar";
import { activeNavItem } from "@/modules/app-shell/lib/nav";

/** 48px, sticky inside the canvas, no blur. The breadcrumb is never the page's <h1>. */
export function AppHeader() {
  const items = useNavItems();
  const headerEnd = useSlot("header-end");
  const { pathname } = useLocation();
  const active = activeNavItem(items, pathname);
  const crumbs = [active?.group, active?.label].filter(Boolean) as string[];

  return (
    <header className="sticky top-0 z-30 flex h-12 shrink-0 items-center gap-2 rounded-t-xl border-b bg-background px-4">
      <SidebarTrigger className="-ml-1" />
      <Separator orientation="vertical" className="mr-1 data-[orientation=vertical]:h-4" />
      <Breadcrumb>
        <BreadcrumbList>
          {crumbs.map((crumb, i) => (
            <Fragment key={crumb}>
              {i > 0 && <BreadcrumbSeparator />}
              <BreadcrumbItem>
                {i === crumbs.length - 1 ? <BreadcrumbPage>{crumb}</BreadcrumbPage> : crumb}
              </BreadcrumbItem>
            </Fragment>
          ))}
        </BreadcrumbList>
      </Breadcrumb>
      {headerEnd.length > 0 && <div className="ml-auto flex items-center gap-2">{headerEnd}</div>}
    </header>
  );
}
