import { useCallback, useEffect, useState } from "react";
import { Pencil, Plus, Trash2, UserPlus } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { ActionMenu } from "@/shared/components/ui/action-menu";
import { ConfirmDialog } from "@/shared/components/ui/confirm-dialog";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { Pagination } from "@/shared/components/ui/pagination";
import { SearchInput } from "@/shared/components/ui/search-input";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import { Toolbar } from "@/shared/components/ui/toolbar";
import { useCan } from "@/shared/access";
import { formatDate } from "@/shared/lib/format";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage } from "@/shared/services/api-error";
import type { PageMeta } from "@/shared/types/api";
import { UserFormSheet } from "@/modules/users/components/UserFormSheet";
import { deleteUser, listUsers } from "@/modules/users/services/users.service";
import type { User } from "@/modules/users/types/user";

export default function UsersPage() {
  const [rows, setRows] = useState<User[]>([]);
  const [meta, setMeta] = useState<PageMeta>({ page: 1, limit: 25, total: 0, total_pages: 0 });
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<User | undefined>();
  const [sheetOpen, setSheetOpen] = useState(false);
  const [deleting, setDeleting] = useState<User | undefined>();
  // Hide what this user may not do; the API refuses it anyway (users.manage).
  const canManage = useCan("users.manage");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await listUsers({ page, limit, q: query });
      setRows(res.data);
      setMeta(res.meta);
    } catch (err) {
      setError(apiErrorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [page, limit, query]);

  useEffect(() => {
    void load();
  }, [load]);

  function openCreate() {
    setEditing(undefined);
    setSheetOpen(true);
  }

  function openEdit(user: User) {
    setEditing(user);
    setSheetOpen(true);
  }

  const columns: DataTableColumn<User>[] = [
    { header: "Nama", render: (u) => <span className="font-medium">{u.name}</span> },
    { header: "Email", render: (u) => <span className="text-muted-foreground">{u.email}</span> },
    {
      header: "Status",
      render: (u) => (u.is_active ? <StatusBadge tone="success">Aktif</StatusBadge> : <StatusBadge tone="muted">Nonaktif</StatusBadge>),
    },
    { header: "Ditambahkan", id: "created", render: (u) => formatDate(u.created_at) },
  ];
  if (canManage) {
    columns.push({
      header: "",
      id: "actions",
      width: 56,
      align: "right",
      render: (u) => (
        <ActionMenu
          variant="ghost"
          size="icon-sm"
          label={`Aksi untuk ${u.name}`}
          items={[
            { key: "edit", label: "Ubah", icon: <Pencil />, onSelect: () => openEdit(u) },
            { key: "delete", label: "Hapus", icon: <Trash2 />, destructive: true, onSelect: () => setDeleting(u) },
          ]}
        />
      ),
    });
  }

  return (
    <Page>
      <PageHeader
        title="Pengguna"
        description="Orang yang bisa masuk ke aplikasi ini."
        actions={
          canManage && (
            <Button onClick={openCreate}>
              <Plus />
              Tambah pengguna
            </Button>
          )
        }
      />
      <section className="overflow-hidden rounded-xl border border-border bg-card">
        <Toolbar
          plain
          search={
            <SearchInput
              value={query}
              placeholder="Cari nama atau email…"
              onSearch={(q) => {
                setPage(1);
                setQuery(q);
              }}
            />
          }
          active={query ? [{ key: "q", label: `Cari: ${query}`, onRemove: () => setQuery("") }] : []}
        />
        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(u) => u.id}
          loading={loading}
          error={error}
          onRetry={load}
          onRowClick={canManage ? openEdit : undefined}
          rowLabel={(u) => `Ubah ${u.name}`}
          caption="Daftar pengguna"
          emptyTitle={query ? "Tidak ada yang cocok" : "Belum ada pengguna"}
          emptyDescription={query ? `Tidak ada pengguna untuk "${query}".` : "Tambahkan orang yang perlu masuk ke aplikasi."}
          emptyAction={
            query ? (
              <Button variant="outline" size="sm" onClick={() => setQuery("")}>
                Hapus pencarian
              </Button>
            ) : (
              canManage && (
                <Button size="sm" onClick={openCreate}>
                  <UserPlus />
                  Tambah pengguna pertama
                </Button>
              )
            )
          }
        />
        {!error && rows.length > 0 && (
          <Pagination
            page={meta.page}
            limit={meta.limit}
            total={meta.total}
            noun="pengguna"
            onPageChange={setPage}
            onLimitChange={(n) => {
              setPage(1);
              setLimit(n);
            }}
          />
        )}
      </section>

      <UserFormSheet open={sheetOpen} onClose={() => setSheetOpen(false)} user={editing} onSaved={() => void load()} />
      <ConfirmDialog
        open={deleting !== undefined}
        onOpenChange={(open) => !open && setDeleting(undefined)}
        title={`Hapus ${deleting?.name ?? "pengguna"}?`}
        description="Pengguna ini tidak bisa masuk lagi. Tindakan ini tidak dapat dibatalkan."
        confirmLabel="Hapus"
        onConfirm={async () => {
          if (!deleting) return;
          await deleteUser(deleting.id);
          notify.success("Pengguna dihapus");
          await load();
        }}
      />
    </Page>
  );
}
