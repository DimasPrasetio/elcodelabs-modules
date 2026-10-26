import { lazy } from "react";
import { LayoutDashboard } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";

const DashboardPage = lazy(() => import("@/modules/dashboard/pages/DashboardPage"));

export const DASHBOARD_PATHS = {
  home: "/dashboard",
} as const;

export const dashboardModule: FrontendModule = {
  id: "dashboard",
  protected: [{ path: DASHBOARD_PATHS.home, element: <DashboardPage /> }],
  nav: [{ label: "Dashboard", to: DASHBOARD_PATHS.home, icon: LayoutDashboard, order: 0 }],
};
