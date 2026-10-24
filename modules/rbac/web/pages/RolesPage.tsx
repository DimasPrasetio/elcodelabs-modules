import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Plus } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import { apiErrorMessage } from "@/shared/services/api-error";
import { RoleFormDialog } from "@/modules/rbac/components/RoleFormDialog";
import { RBAC_PATHS } from "@/modules/rbac/paths";
import { fetchRoles } from "@/modules/rbac/services/rbac.service";
import { WILDCARD } from "@/modules/rbac/types/rbac";
import type { Role } from "@/modules/rbac/types/rbac";

export default function RolesPage() {
  const navigate = useNavigate();
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setRoles(await fetchRoles());
    } catch (err) {
      setError(apiErrorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const columns: DataTableColumn<Role>[] = [
    {
      header: "Peran",
      render: (r) => (
        <span className="flex items-center gap-2 font-medium">
          {r.name}
          {r.is_system && <StatusBadge tone="info">Sistem</StatusBadge>}
        </span>
      ),
    },
    { header: "Keterangan", render: (r) => <span className="text-muted-foreground">{r.description || "–"}</span> },
    { header: "Izin", align: "right", width: 96, render: (r) => (r.permissions.includes(WILDCARD) ? "Semua" : r.permissions.length) },
    { header: "Anggota", align: "right", width: 96, render: (r) => r.member_count },
  ];

  return (
    <Page>
      <PageHeader
        title="Peran"
        description="Apa yang boleh dilakukan setiap kelompok orang. Daftar izin berasal dari modul yang terpasang."
        actions={
          <Button onClick={() => setCreating(true)}>
            <Plus />
            Peran baru
          </Button>
        }
      />
      <section className="overflow-hidden rounded-xl border border-border bg-card">
        <DataTable
          columns={columns}
          rows={roles}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={load}
          onRowClick={(r) => navigate(RBAC_PATHS.role(r.id))}
          rowLabel={(r) => `Buka ${r.name}`}
          caption="Daftar peran"
          emptyTitle="Belum ada peran"
        />
      </section>
      <RoleFormDialog open={creating} onClose={() => setCreating(false)} onSaved={(r) => navigate(RBAC_PATHS.role(r.id))} />
    </Page>
  );
}
