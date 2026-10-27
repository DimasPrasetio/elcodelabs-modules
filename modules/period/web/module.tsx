import { lazy } from "react";
import { CalendarCheck } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";

const PeriodPage = lazy(() => import("@/modules/period/pages/PeriodPage"));

export const PERIOD_PATHS = {
  root: "/period",
} as const;

export const periodModule: FrontendModule = {
  id: "period",
  protected: [{ path: PERIOD_PATHS.root, element: <PeriodPage /> }],
  nav: [{ label: "Tutup buku", to: PERIOD_PATHS.root, icon: CalendarCheck, group: "Pengaturan", order: 820, permission: "period.read" }],
};
