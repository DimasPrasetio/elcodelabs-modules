import { lazy } from "react";
import { ShieldCheck } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";
import { RbacAccessProvider } from "@/modules/rbac/access/RbacAccessProvider";
import { RBAC_PATHS } from "@/modules/rbac/paths";

const RolesPage = lazy(() => import("@/modules/rbac/pages/RolesPage"));
const RoleDetailPage = lazy(() => import("@/modules/rbac/pages/RoleDetailPage"));

export { RBAC_PATHS };

export const rbacModule: FrontendModule = {
  id: "rbac",
  protected: [
    { path: RBAC_PATHS.roles, element: <RolesPage /> },
    { path: "/roles/:id", element: <RoleDetailPage /> },
  ],
  nav: [{ label: "Peran", to: RBAC_PATHS.roles, icon: ShieldCheck, group: "Pengaturan", order: 910, permission: "rbac.manage" }],
  provider: RbacAccessProvider,
};
