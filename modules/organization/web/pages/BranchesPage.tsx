import { useState } from "react";
import { Pencil, Plus, Power } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { ActionMenu } from "@/shared/components/ui/action-menu";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { Page } from "@/shared/components/ui/page";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import { ConfirmDialog } from "@/shared/components/ui/confirm-dialog";
import { useResource } from "@/shared/hooks/use-resource";
import { useCan } from "@/shared/access";
import { notify } from "@/shared/lib/notify";
import { BranchFormDialog } from "@/modules/organization/components/BranchFormDialog";
import { OrganizationHeader } from "@/modules/organization/components/OrganizationHeader";
import { useOrganization } from "@/modules/organization/context/OrganizationProvider";
import { fetchBranches, setBranchActive } from "@/modules/organization/services/organization.service";
import { zoneLabel } from "@/modules/organization/types/organization";
import type { Branch } from "@/modules/organization/types/organization";

export default function BranchesPage() {
  const { me, reload: reloadMe } = useOrganization();
  // Branches are the whole company's business: managed from the company as a whole.
  const canManage = useCan("organization.manage") && me?.current.branch_id === 0;
  const branches = useResource((signal) => fetchBranches(signal), []);
  const [editing, setEditing] = useState<Branch | "new" | null>(null);
  const [toggling, setToggling] = useState<Branch | null>(null);

  const columns: DataTableColumn<Branch>[] = [
    { header: "Kode", width: 120, render: (b) => <span className="font-mono text-sm">{b.code}</span> },
    { header: "Nama", render: (b) => <span className="font-medium">{b.name}</span> },
    { header: "Zona waktu", render: (b) => (b.timezone ? zoneLabel(b.timezone) : <span className="text-muted-foreground">Ikut perusahaan</span>) },
    { header: "NITKU", defaultHidden: true, render: (b) => <span className="font-mono text-sm">{b.nitku || "–"}</span> },
    { header: "Status", width: 120, render: (b) => (b.is_active ? <StatusBadge tone="success">Aktif</StatusBadge> : <StatusBadge tone="muted">Nonaktif</StatusBadge>) },
    ...(canManage
      ? [
          {
            header: "",
            width: 56,
            align: "right" as const,
            render: (b: Branch) => (
              <ActionMenu
                variant="ghost"
                size="icon-sm"
                items={[
                  { key: "edit", label: "Ubah", icon: <Pencil />, onSelect: () => setEditing(b) },
                  { key: "toggle", label: b.is_active ? "Nonaktifkan" : "Aktifkan", icon: <Power />, destructive: b.is_active, onSelect: () => setToggling(b) },
                ]}
              />
            ),
          },
        ]
      : []),
  ];

  return (
    <Page>
      <OrganizationHeader
        description="Tempat kegiatan usaha perusahaan ini. Orang yang ditempatkan di satu cabang hanya bekerja di cabang itu."
        actions={
          canManage && (
            <Button onClick={() => setEditing("new")}>
              <Plus />
              Cabang baru
            </Button>
          )
        }
      />
      <section className="overflow-hidden rounded-xl border border-border bg-card">
        <DataTable
          columns={columns}
          rows={branches.data ?? []}
          rowKey={(b) => b.id}
          loading={branches.loading}
          error={branches.error}
          onRetry={branches.reload}
          caption="Daftar cabang"
          emptyTitle="Belum ada cabang"
          emptyDescription="Tanpa cabang, semua orang bekerja untuk seluruh perusahaan."
        />
      </section>
      <BranchFormDialog
        open={editing !== null}
        onClose={() => setEditing(null)}
        branch={editing === "new" || editing === null ? undefined : editing}
        onSaved={() => {
          branches.reload();
          reloadMe();
        }}
      />
      <ConfirmDialog
        open={toggling !== null}
        onOpenChange={(open) => !open && setToggling(null)}
        title={toggling?.is_active ? `Nonaktifkan ${toggling?.name}?` : `Aktifkan ${toggling?.name}?`}
        description={
          toggling?.is_active
            ? "Orang yang hanya ditempatkan di cabang ini tidak bisa bekerja sampai cabang diaktifkan lagi. Datanya tetap ada."
            : "Orang yang ditempatkan di cabang ini bisa bekerja lagi di sana."
        }
        confirmLabel={toggling?.is_active ? "Nonaktifkan" : "Aktifkan"}
        onConfirm={async () => {
          if (!toggling) return;
          await setBranchActive(toggling.id, !toggling.is_active);
          notify.success(toggling.is_active ? "Cabang dinonaktifkan" : "Cabang diaktifkan");
          branches.reload();
          reloadMe();
        }}
      />
    </Page>
  );
}
