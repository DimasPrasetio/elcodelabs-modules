import { lazy } from "react";
import { History } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";

const AuditPage = lazy(() => import("@/modules/audit-log/pages/AuditPage"));

export const AUDIT_PATHS = {
  list: "/audit",
} as const;

export const auditLogModule: FrontendModule = {
  id: "audit-log",
  protected: [{ path: AUDIT_PATHS.list, element: <AuditPage /> }],
  nav: [{ label: "Log aktivitas", to: AUDIT_PATHS.list, icon: History, group: "Pengaturan", order: 980, permission: "audit-log.read" }],
};
