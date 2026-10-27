import type { ReactNode } from "react";
import { NavTabs } from "@/shared/components/ui/nav-tabs";
import { PageHeader } from "@/shared/components/ui/page-header";
import { useCan } from "@/shared/access";
import { useOrganization } from "@/modules/organization/context/OrganizationProvider";
import { ORGANIZATION_PATHS } from "@/modules/organization/paths";

/** The name of the company this tab works in. */
export function useCurrentCompanyName(): string {
  const { me } = useOrganization();
  return me?.units.find((u) => u.company_id === me.current.company_id)?.company_name ?? "perusahaan ini";
}

export function OrganizationHeader({ description, actions }: { description: ReactNode; actions?: ReactNode }) {
  const canCompanies = useCan("organization.companies");
  const company = useCurrentCompanyName();
  return (
    <div className="grid gap-4">
      <PageHeader title={`Organisasi — ${company}`} description={description} actions={actions} />
      <NavTabs
        label="Bagian organisasi"
        items={[
          { to: ORGANIZATION_PATHS.branches, label: "Cabang" },
          { to: ORGANIZATION_PATHS.placements, label: "Penempatan" },
          ...(canCompanies ? [{ to: ORGANIZATION_PATHS.companies, label: "Perusahaan" }] : []),
        ]}
      />
    </div>
  );
}
