import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, Pencil, ShieldCheck, Trash2, UserPlus, X } from "lucide-react";
import { Alert, AlertDescription } from "@/shared/components/primitives/alert";
import { Badge } from "@/shared/components/primitives/badge";
import { Button } from "@/shared/components/primitives/button";
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/shared/components/primitives/card";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import { Skeleton } from "@/shared/components/primitives/skeleton";
import { Spinner } from "@/shared/components/primitives/spinner";
import { ConfirmDialog } from "@/shared/components/ui/confirm-dialog";
import { EmptyState } from "@/shared/components/ui/empty-state";
import { ErrorState } from "@/shared/components/ui/error-state";
import { PageHeader } from "@/shared/components/ui/page-header";
import { apiErrorMessage } from "@/shared/services/api-error";
import { AddMemberDialog } from "@/modules/rbac/components/AddMemberDialog";
import { RoleFormDialog } from "@/modules/rbac/components/RoleFormDialog";
import { RBAC_PATHS } from "@/modules/rbac/paths";
import {
  deleteRole,
  fetchCatalog,
  fetchMembers,
  fetchRole,
  removeMember,
  updateRole,
} from "@/modules/rbac/services/rbac.service";
import { WILDCARD } from "@/modules/rbac/types/rbac";
import type { Permission, RbacUser, Role } from "@/modules/rbac/types/rbac";

function byModule(catalog: Permission[]): [string, Permission[]][] {
  const groups = new Map<string, Permission[]>();
  for (const p of catalog) groups.set(p.module, [...(groups.get(p.module) ?? []), p]);
  return [...groups.entries()].sort(([a], [b]) => a.localeCompare(b));
}

export default function RoleDetailPage() {
  const id = Number(useParams().id);
  const navigate = useNavigate();
  const [role, setRole] = useState<Role | null>(null);
  const [catalog, setCatalog] = useState<Permission[]>([]);
  const [members, setMembers] = useState<RbacUser[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [error, setError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [adding, setAdding] = useState(false);
  const [removing, setRemoving] = useState<RbacUser | null>(null);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [r, c, m] = await Promise.all([fetchRole(id), fetchCatalog(), fetchMembers(id)]);
      setRole(r);
      setCatalog(c);
      setMembers(m);
      setSelected(new Set(r.permissions));
    } catch (err) {
      setError(apiErrorMessage(err));
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  const dirty = useMemo(() => {
    if (!role) return false;
    const before = new Set(role.permissions);
    return before.size !== selected.size || [...selected].some((p) => !before.has(p));
  }, [role, selected]);

  function toggle(name: string, on: boolean) {
    setSelected((s) => {
      const next = new Set(s);
      if (on) next.add(name);
      else next.delete(name);
      return next;
    });
  }

  async function savePermissions() {
    if (!role) return;
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateRole(role.id, { name: role.name, description: role.description, permissions: [...selected] });
      setRole(updated);
      setSelected(new Set(updated.permissions));
    } catch (err) {
      setSaveError(apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  if (error) return <ErrorState message={error} onRetry={load} />;
  if (!role) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  const all = role.permissions.includes(WILDCARD);

  return (
    <>
      <Button variant="ghost" size="sm" asChild className="-ml-2 mb-2">
        <Link to={RBAC_PATHS.roles}>
          <ArrowLeft />
          Roles
        </Link>
      </Button>
      <PageHeader
        title={role.name}
        description={role.description || undefined}
        meta={role.is_system && <Badge variant="secondary">System</Badge>}
        actions={
          !role.is_system && (
            <>
              <Button variant="outline" onClick={() => setDeleting(true)}>
                <Trash2 />
                Delete
              </Button>
              <Button variant="outline" onClick={() => setEditing(true)}>
                <Pencil />
                Edit
              </Button>
            </>
          )
        }
      />

      <div className="grid gap-6 lg:grid-cols-[3fr_2fr]">
        <Card>
          <CardHeader>
            <CardTitle>Permissions</CardTitle>
            <CardDescription>
              {all ? "This role holds every permission, including ones modules add later." : "What members of this role may do, by module."}
            </CardDescription>
            {!all && (
              <CardAction>
                <Button size="sm" disabled={!dirty || saving} onClick={savePermissions}>
                  {saving && <Spinner />}
                  Save
                </Button>
              </CardAction>
            )}
          </CardHeader>
          <CardContent className="space-y-6">
            {saveError && (
              <Alert variant="destructive">
                <AlertDescription>{saveError}</AlertDescription>
              </Alert>
            )}
            {byModule(catalog).map(([module, perms]) => (
              <fieldset key={module} className="space-y-3">
                <legend className="text-2xs font-medium tracking-wide text-muted-foreground uppercase">{module}</legend>
                {perms.map((p) => (
                  <label key={p.name} className="flex items-start gap-3">
                    <Checkbox
                      className="mt-0.5"
                      checked={all || selected.has(p.name)}
                      disabled={all}
                      onCheckedChange={(v) => toggle(p.name, v === true)}
                    />
                    <span className="grid gap-0.5">
                      <span className="text-sm">{p.description}</span>
                      <span className="font-mono text-xs text-muted-foreground">{p.name}</span>
                    </span>
                  </label>
                ))}
              </fieldset>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Members</CardTitle>
            <CardDescription>{members.length === 1 ? "1 user" : `${members.length} users`}</CardDescription>
            <CardAction>
              <Button size="sm" variant="outline" onClick={() => setAdding(true)}>
                <UserPlus />
                Add
              </Button>
            </CardAction>
          </CardHeader>
          <CardContent>
            {members.length === 0 ? (
              <EmptyState icon={ShieldCheck} title="Nobody has this role yet" />
            ) : (
              <ul className="divide-y">
                {members.map((m) => (
                  <li key={m.id} className="flex items-center justify-between gap-3 py-2.5">
                    <div className="min-w-0">
                      <div className="truncate text-sm font-medium">{m.missing ? `User #${m.id}` : m.name}</div>
                      <div className="truncate text-xs text-muted-foreground">{m.missing ? "No longer exists" : m.email}</div>
                    </div>
                    <Button variant="ghost" size="icon-sm" aria-label={`Remove ${m.name || `user #${m.id}`}`} onClick={() => setRemoving(m)}>
                      <X />
                    </Button>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>

      <RoleFormDialog open={editing} onOpenChange={setEditing} role={role} onSaved={(r) => setRole(r)} />
      <AddMemberDialog
        open={adding}
        onOpenChange={setAdding}
        roleId={role.id}
        roleName={role.name}
        memberIds={new Set(members.map((m) => m.id))}
        onAdded={() => void fetchMembers(role.id).then(setMembers)}
      />
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
        title={`Take ${role.name} from ${removing?.name || "this user"}?`}
        description="They lose its permissions on their next request."
        confirmLabel="Remove"
        destructive
        onConfirm={async () => {
          if (!removing) return;
          await removeMember(role.id, removing.id);
          setMembers(await fetchMembers(role.id));
        }}
      />
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={`Delete ${role.name}?`}
        description="Its members lose its permissions. This cannot be undone."
        confirmLabel="Delete role"
        destructive
        onConfirm={async () => {
          await deleteRole(role.id);
          navigate(RBAC_PATHS.roles, { replace: true });
        }}
      />
    </>
  );
}
