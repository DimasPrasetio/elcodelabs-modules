import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Plus, ShieldCheck } from "lucide-react";
import { Badge } from "@/shared/components/primitives/badge";
import { Button } from "@/shared/components/primitives/button";
import { Card } from "@/shared/components/primitives/card";
import { DataTable } from "@/shared/components/ui/data-table";
import type { Column } from "@/shared/components/ui/data-table";
import { EmptyState } from "@/shared/components/ui/empty-state";
import { ErrorState } from "@/shared/components/ui/error-state";
import { PageHeader } from "@/shared/components/ui/page-header";
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

  const columns: Column<Role>[] = [
    {
      key: "name",
      header: "Role",
      cell: (r) => (
        <span className="flex items-center gap-2 font-medium">
          {r.name}
          {r.is_system && <Badge variant="secondary">System</Badge>}
        </span>
      ),
    },
    { key: "description", header: "Description", cell: (r) => <span className="text-muted-foreground">{r.description || "—"}</span> },
    {
      key: "permissions",
      header: "Permissions",
      align: "right",
      cell: (r) => (r.permissions.includes(WILDCARD) ? "All" : r.permissions.length),
    },
    { key: "members", header: "Members", align: "right", cell: (r) => r.member_count },
  ];

  return (
    <>
      <PageHeader
        title="Roles"
        description="What each group of people may do. Permissions come from the installed modules."
        actions={
          <Button onClick={() => setCreating(true)}>
            <Plus />
            New role
          </Button>
        }
      />
      <Card className="gap-0 overflow-hidden py-0">
        {error ? (
          <ErrorState message={error} onRetry={load} />
        ) : (
          <DataTable
            columns={columns}
            rows={roles}
            getRowId={(r) => r.id}
            loading={loading}
            onRowClick={(r) => navigate(RBAC_PATHS.role(r.id))}
            rowLabel={(r) => `Open ${r.name}`}
            empty={<EmptyState icon={ShieldCheck} title="No roles yet" />}
          />
        )}
      </Card>
      <RoleFormDialog open={creating} onOpenChange={setCreating} onSaved={(r) => navigate(RBAC_PATHS.role(r.id))} />
    </>
  );
}
