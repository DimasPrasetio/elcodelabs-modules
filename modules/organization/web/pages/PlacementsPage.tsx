import { useMemo, useState } from "react";
import { UserPlus, X } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { Page } from "@/shared/components/ui/page";
import { SectionCard } from "@/shared/components/ui/section-card";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import { ConfirmDialog } from "@/shared/components/ui/confirm-dialog";
import { useResource } from "@/shared/hooks/use-resource";
import { useCan } from "@/shared/access";
import { notify } from "@/shared/lib/notify";
import { OrganizationHeader, useCurrentCompanyName } from "@/modules/organization/components/OrganizationHeader";
import { PlaceUserDialog } from "@/modules/organization/components/PlaceUserDialog";
import { useOrganization } from "@/modules/organization/context/OrganizationProvider";
import { fetchBranches, fetchPeople, fetchUnplaced, unplaceUser } from "@/modules/organization/services/organization.service";
import type { OrgUser, Person } from "@/modules/organization/types/organization";

export default function PlacementsPage() {
  const canManage = useCan("organization.manage");
  const canCompanies = useCan("organization.companies");
  const { me } = useOrganization();
  const atHome = me?.current.company_id === 1 && me?.current.branch_id === 0;
  const inBranch = me?.current.branch_id || undefined;
  const company = useCurrentCompanyName();
  const people = useResource((signal) => fetchPeople(signal), []);
  const branches = useResource((signal) => fetchBranches(signal), []);
  const unplaced = useResource((signal) => (canCompanies && atHome ? fetchUnplaced(signal) : Promise.resolve([])), [canCompanies, atHome]);
  const [placing, setPlacing] = useState<OrgUser | "search" | null>(null);
  const [removing, setRemoving] = useState<Person | null>(null);

  const branchName = useMemo(() => new Map((branches.data ?? []).map((b) => [b.id, b.name])), [branches.data]);
  const where = (p: Person) => (p.branch_id === 0 ? "Seluruh perusahaan" : (branchName.get(p.branch_id) ?? `Cabang #${p.branch_id}`));

  const columns: DataTableColumn<Person>[] = [
    {
      header: "Nama",
      render: (p) => (
        <div className="min-w-0">
          <div className="truncate font-medium">{p.missing ? `Pengguna #${p.id}` : p.name}</div>
          <div className="truncate text-xs text-muted-foreground">{p.missing ? "Sudah tidak ada" : p.email}</div>
        </div>
      ),
    },
    {
      header: "Ditempatkan di",
      render: (p) => (
        <span className="flex items-center gap-2">
          {where(p)}
          {p.is_default && <StatusBadge tone="neutral">Default</StatusBadge>}
          {!p.missing && !p.is_active && <StatusBadge tone="muted">Nonaktif</StatusBadge>}
        </span>
      ),
    },
    ...(canManage
      ? [
          {
            header: "",
            width: 56,
            align: "right" as const,
            // Working in one branch reaches only that branch's placements.
            render: (p: Person) =>
              (inBranch === undefined || p.branch_id === inBranch) && (
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={`Cabut penempatan ${p.name || `pengguna #${p.id}`}`}
                  className="text-muted-foreground hover:text-destructive"
                  onClick={() => setRemoving(p)}
                >
                  <X />
                </Button>
              ),
          },
        ]
      : []),
  ];

  return (
    <Page>
      <OrganizationHeader
        description="Siapa yang bekerja di perusahaan ini, dan di mana. Apa yang boleh mereka lakukan di sana diatur lewat Peran."
        actions={
          canManage && (
            <Button onClick={() => setPlacing("search")}>
              <UserPlus />
              Tempatkan pengguna
            </Button>
          )
        }
      />
      <section className="overflow-hidden rounded-xl border border-border bg-card">
        <DataTable
          columns={columns}
          rows={people.data ?? []}
          rowKey={(p) => `${p.id}:${p.branch_id}`}
          loading={people.loading}
          error={people.error}
          onRetry={people.reload}
          caption="Daftar penempatan"
          emptyTitle="Belum ada yang ditempatkan di sini"
        />
      </section>
      {(unplaced.data ?? []).length > 0 && (
        <SectionCard
          title="Tidak bisa bekerja di mana pun"
          description="Akun-akun ini belum ditempatkan, atau hanya ditempatkan di perusahaan atau cabang yang nonaktif."
          flush
        >
          <ul className="divide-y divide-border">
            {(unplaced.data ?? []).map((u) => (
              <li key={u.id} className="flex items-center justify-between gap-3 px-5 py-3">
                <div className="min-w-0">
                  <div className="truncate text-sm font-medium">{u.name}</div>
                  <div className="truncate text-xs text-muted-foreground">{u.email}</div>
                </div>
                {canManage && (
                  <Button size="sm" variant="outline" onClick={() => setPlacing(u)}>
                    Tempatkan
                  </Button>
                )}
              </li>
            ))}
          </ul>
        </SectionCard>
      )}
      <PlaceUserDialog
        open={placing !== null}
        onClose={() => setPlacing(null)}
        companyName={company}
        branches={branches.data ?? []}
        onlyBranch={inBranch}
        user={placing === "search" || placing === null ? undefined : placing}
        onPlaced={() => {
          people.reload();
          unplaced.reload();
        }}
      />
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
        title={`Cabut penempatan ${removing?.name || "pengguna ini"}?`}
        description={`${removing ? where(removing) : ""}: ia tidak bisa bekerja di sini lagi mulai permintaan berikutnya. Akun dan perannya tetap ada.`}
        confirmLabel="Cabut penempatan"
        onConfirm={async () => {
          if (!removing) return;
          await unplaceUser(removing.id, removing.branch_id);
          notify.success("Penempatan dicabut");
          people.reload();
          unplaced.reload();
        }}
      />
    </Page>
  );
}
