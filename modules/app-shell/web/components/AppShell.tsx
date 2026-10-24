import { Outlet } from "react-router-dom";
import { SidebarInset, SidebarProvider } from "@/shared/components/primitives/sidebar";
import { TooltipProvider } from "@/shared/components/primitives/tooltip";
import { AppHeader } from "@/modules/app-shell/components/AppHeader";
import { AppSidebar } from "@/modules/app-shell/components/AppSidebar";

/**
 * The frame around every protected page: a navy sidebar and an inset canvas that holds the
 * header and the page. Navigation and the user menu come from other modules through
 * @/shared/composition — the shell imports none of them.
 */
export function AppShell() {
  return (
    <TooltipProvider delayDuration={0}>
      <SidebarProvider>
        <AppSidebar />
        <SidebarInset>
          <AppHeader />
          <div className="mx-auto w-full max-w-[1440px] flex-1 p-4 md:p-6">
            <Outlet />
          </div>
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  );
}
