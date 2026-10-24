import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, Pencil, ShieldCheck, Trash2, UserPlus, X } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import { ConfirmDialog } from "@/shared/components/ui/confirm-dialog";
import { EmptyState } from "@/shared/components/ui/empty-state";
import { ErrorState } from "@/shared/components/ui/error-state";
import { Notice } from "@/shared/components/ui/notice";
import { Page, PageColumns } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { SectionCard } from "@/shared/components/ui/section-card";
import { DetailSkeleton } from "@/shared/components/ui/skeletons";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage } from "@/shared/services/api-error";
import { AddMemberDialog } from "@/modules/rbac/components/AddMemberDialog";
import { RoleFormDialog } from "@/modules/rbac/components/RoleFormDialog";
import { RBAC_PATHS } from "@/modules/rbac/paths";
import { deleteRole, fetchCatalog, fetchMembers, fetchRole, removeMember, updateRole } from "@/modules/rbac/services/rbac.service";
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
      notify.success("Izin peran disimpan");
    } catch (err) {
      setSaveError(apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  if (error) return <ErrorState message={error} onRetry={load} />;
  if (!role) return <DetailSkeleton />;

  const all = role.permissions.includes(WILDCARD);

  return (
    <Page>
      <div>
        <Button variant="ghost" size="sm" asChild className="-ml-2 mb-2">
          <Link to={RBAC_PATHS.roles}>
            <ArrowLeft />
            Peran
          </Link>
        </Button>
        <PageHeader
          title={role.name}
          description={role.description || undefined}
          meta={role.is_system && <StatusBadge tone="info">Sistem</StatusBadge>}
          actions={
            !role.is_system && (
              <>
                <Button variant="outline" onClick={() => setDeleting(true)}>
                  <Trash2 />
                  Hapus
                </Button>
                <Button variant="outline" onClick={() => setEditing(true)}>
                  <Pencil />
                  Ubah
                </Button>
              </>
            )
          }
        />
      </div>

      <PageColumns
        main={
          <SectionCard
            title="Izin"
            description={all ? "Peran ini memegang semua izin, termasuk yang ditambahkan modul kemudian." : "Apa yang boleh dilakukan anggota peran ini, per modul."}
            actions={
              !all && (
                <Button size="sm" disabled={!dirty} loading={saving} onClick={savePermissions}>
                  Simpan izin
                </Button>
              )
            }
          >
            <div className="space-y-6">
              {saveError && <Notice tone="danger" title="Gagal menyimpan">{saveError}</Notice>}
              {byModule(catalog).map(([module, perms]) => (
                <fieldset key={module} className="space-y-3">
                  <legend className="text-2xs font-semibold tracking-wide text-muted-foreground uppercase">{module}</legend>
                  {perms.map((p) => (
                    <label key={p.name} className="flex items-start gap-3">
                      <Checkbox className="mt-0.5" checked={all || selected.has(p.name)} disabled={all} onCheckedChange={(v) => toggle(p.name, v === true)} />
                      <span className="grid gap-0.5">
                        <span className="text-sm">{p.description}</span>
                        <span className="font-mono text-xs text-muted-foreground">{p.name}</span>
                      </span>
                    </label>
                  ))}
                </fieldset>
              ))}
            </div>
          </SectionCard>
        }
        aside={
          <SectionCard
            title="Anggota"
            description={`${members.length} pengguna`}
            flush
            actions={
              <Button size="sm" variant="outline" onClick={() => setAdding(true)}>
                <UserPlus />
                Tambah
              </Button>
            }
          >
            {members.length === 0 ? (
              <EmptyState icon={ShieldCheck} title="Belum ada yang memegang peran ini" />
            ) : (
              <ul className="divide-y divide-border">
                {members.map((m) => (
                  <li key={m.id} className="flex items-center justify-between gap-3 px-5 py-3">
                    <div className="min-w-0">
                      <div className="truncate text-sm font-medium">{m.missing ? `Pengguna #${m.id}` : m.name}</div>
                      <div className="truncate text-xs text-muted-foreground">{m.missing ? "Sudah tidak ada" : m.email}</div>
                    </div>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={`Cabut peran dari ${m.name || `pengguna #${m.id}`}`}
                      className="text-muted-foreground hover:text-destructive"
                      onClick={() => setRemoving(m)}
                    >
                      <X />
                    </Button>
                  </li>
                ))}
              </ul>
            )}
          </SectionCard>
        }
      />

      <RoleFormDialog open={editing} onClose={() => setEditing(false)} role={role} onSaved={setRole} />
      <AddMemberDialog
        open={adding}
        onClose={() => setAdding(false)}
        roleId={role.id}
        roleName={role.name}
        memberIds={new Set(members.map((m) => m.id))}
        onAdded={() => void fetchMembers(role.id).then(setMembers)}
      />
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
        title={`Cabut ${role.name} dari ${removing?.name || "pengguna ini"}?`}
        description="Izin dari peran ini hilang pada permintaan berikutnya."
        confirmLabel="Cabut peran"
        onConfirm={async () => {
          if (!removing) return;
          await removeMember(role.id, removing.id);
          notify.success("Peran dicabut");
          setMembers(await fetchMembers(role.id));
        }}
      />
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={`Hapus peran ${role.name}?`}
        description="Anggotanya kehilangan izin dari peran ini. Tindakan ini tidak dapat dibatalkan."
        confirmLabel="Hapus peran"
        onConfirm={async () => {
          await deleteRole(role.id);
          notify.success("Peran dihapus");
          navigate(RBAC_PATHS.roles, { replace: true });
        }}
      />
    </Page>
  );
}
