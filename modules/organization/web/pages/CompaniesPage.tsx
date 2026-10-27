import { useState } from "react";
import { Pencil, Plus, Power, UserPlus } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { ActionMenu } from "@/shared/components/ui/action-menu";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { Notice } from "@/shared/components/ui/notice";
import { Page } from "@/shared/components/ui/page";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import { ConfirmDialog } from "@/shared/components/ui/confirm-dialog";
import { useResource } from "@/shared/hooks/use-resource";
import { notify } from "@/shared/lib/notify";
import { setScope } from "@/shared/scope";
import { CompanyFormDialog } from "@/modules/organization/components/CompanyFormDialog";
import { CompanyPlaceDialog } from "@/modules/organization/components/CompanyPlaceDialog";
import { OrganizationHeader } from "@/modules/organization/components/OrganizationHeader";
import { useOrganization } from "@/modules/organization/context/OrganizationProvider";
import { fetchCompanies, setCompanyActive } from "@/modules/organization/services/organization.service";
import { zoneLabel } from "@/modules/organization/types/organization";
import type { Company } from "@/modules/organization/types/organization";

/** Companies are managed from the home company as a whole (company 1). */
export default function CompaniesPage() {
  const { me, reload: reloadMe } = useOrganization();
  const atHome = me?.current.company_id === 1 && me?.current.branch_id === 0;
  const companies = useResource((signal) => (atHome ? fetchCompanies(signal) : Promise.resolve([])), [atHome]);
  const [editing, setEditing] = useState<Company | "new" | null>(null);
  const [toggling, setToggling] = useState<Company | null>(null);
  const [placing, setPlacing] = useState<Company | null>(null);

  const columns: DataTableColumn<Company>[] = [
    { header: "Kode", width: 140, render: (c) => <span className="font-mono text-sm">{c.code}</span> },
    {
      header: "Nama",
      render: (c) => (
        <span className="flex items-center gap-2 font-medium">
          {c.name}
          {c.id === 1 && <StatusBadge tone="info">Induk</StatusBadge>}
        </span>
      ),
    },
    { header: "Zona waktu", render: (c) => zoneLabel(c.timezone) },
    { header: "Status", width: 120, render: (c) => (c.is_active ? <StatusBadge tone="success">Aktif</StatusBadge> : <StatusBadge tone="muted">Nonaktif</StatusBadge>) },
    {
      header: "",
      width: 56,
      align: "right",
      render: (c) => (
        <ActionMenu
          variant="ghost"
          size="icon-sm"
          items={[
            { key: "edit", label: "Ubah", icon: <Pencil />, onSelect: () => setEditing(c) },
            { key: "place", label: "Tempatkan pengguna…", icon: <UserPlus />, onSelect: () => setPlacing(c) },
            ...(c.id === 1
              ? []
              : [{ key: "toggle", label: c.is_active ? "Nonaktifkan" : "Aktifkan", icon: <Power />, destructive: c.is_active, onSelect: () => setToggling(c) }]),
          ]}
        />
      ),
    },
  ];

  return (
    <Page>
      <OrganizationHeader
        description="Badan hukum yang dilayani aplikasi ini. Data setiap perusahaan terpisah; orang hanya bekerja di perusahaan tempat ia ditempatkan."
        actions={
          atHome && (
            <Button onClick={() => setEditing("new")}>
              <Plus />
              Perusahaan baru
            </Button>
          )
        }
      />
      {!atHome ? (
        <Notice tone="info" title="Perusahaan dikelola dari perusahaan induk">
          <span>Pindah ke seluruh perusahaan induk untuk membuat atau mengubah perusahaan. </span>
          <Button variant="link" className="h-auto p-0" onClick={() => setScope("1")}>
            Pindah sekarang
          </Button>
        </Notice>
      ) : (
        <section className="overflow-hidden rounded-xl border border-border bg-card">
          <DataTable
            columns={columns}
            rows={companies.data ?? []}
            rowKey={(c) => c.id}
            loading={companies.loading}
            error={companies.error}
            onRetry={companies.reload}
            caption="Daftar perusahaan"
            emptyTitle="Belum ada perusahaan"
          />
        </section>
      )}
      <CompanyFormDialog
        open={editing !== null}
        onClose={() => setEditing(null)}
        company={editing === "new" || editing === null ? undefined : editing}
        onSaved={() => {
          companies.reload();
          reloadMe();
        }}
      />
      <CompanyPlaceDialog company={placing} onClose={() => setPlacing(null)} />
      <ConfirmDialog
        open={toggling !== null}
        onOpenChange={(open) => !open && setToggling(null)}
        title={toggling?.is_active ? `Nonaktifkan ${toggling?.name}?` : `Aktifkan ${toggling?.name}?`}
        description={
          toggling?.is_active
            ? "Tidak ada yang bisa bekerja di perusahaan ini dan pekerjaan latarnya berhenti sampai diaktifkan lagi. Datanya tetap ada."
            : "Orang yang ditempatkan di perusahaan ini bisa bekerja lagi di sana."
        }
        confirmLabel={toggling?.is_active ? "Nonaktifkan" : "Aktifkan"}
        onConfirm={async () => {
          if (!toggling) return;
          await setCompanyActive(toggling.id, !toggling.is_active);
          notify.success(toggling.is_active ? "Perusahaan dinonaktifkan" : "Perusahaan diaktifkan");
          companies.reload();
          reloadMe();
        }}
      />
    </Page>
  );
}
