import { lazy } from "react";
import { Building2 } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";
import { ScopePicker } from "@/modules/organization/components/ScopePicker";
import { OrganizationProvider } from "@/modules/organization/context/OrganizationProvider";
import { ORGANIZATION_PATHS } from "@/modules/organization/paths";

const BranchesPage = lazy(() => import("@/modules/organization/pages/BranchesPage"));
const PlacementsPage = lazy(() => import("@/modules/organization/pages/PlacementsPage"));
const CompaniesPage = lazy(() => import("@/modules/organization/pages/CompaniesPage"));

export { ORGANIZATION_PATHS };

export const organizationModule: FrontendModule = {
  id: "organization",
  protected: [
    { path: ORGANIZATION_PATHS.branches, element: <BranchesPage /> },
    { path: ORGANIZATION_PATHS.placements, element: <PlacementsPage /> },
    { path: ORGANIZATION_PATHS.companies, element: <CompaniesPage /> },
  ],
  nav: [{ label: "Organisasi", to: ORGANIZATION_PATHS.branches, icon: Building2, group: "Pengaturan", order: 905, permission: "organization.read" }],
  slots: { "shell:scope": <ScopePicker /> },
  provider: OrganizationProvider,
};
