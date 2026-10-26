import { useState } from "react";
import { Pencil, Plus, UserCheck, UserPlus, UserX } from "lucide-react";
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
import { useResource } from "@/shared/hooks/use-resource";
import { formatDate } from "@/shared/lib/format";
import { notify } from "@/shared/lib/notify";
import { UserFormSheet } from "@/modules/users/components/UserFormSheet";
import { listUsers, setUserActive } from "@/modules/users/services/users.service";
import type { User } from "@/modules/users/types/user";

export default function UsersPage() {
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [query, setQuery] = useState("");
  const [editing, setEditing] = useState<User | undefined>();
  const [sheetOpen, setSheetOpen] = useState(false);
  // People are deactivated, never deleted: records elsewhere keep pointing at them.
  const [toggling, setToggling] = useState<User | undefined>();
  // Hide what this user may not do; the API refuses it anyway (users.manage).
  const canManage = useCan("users.manage");

  // Only the latest answer counts: typing a new search aborts the request for the old one.
  const users = useResource((signal) => listUsers({ page, limit, q: query }, signal), [page, limit, query]);
  const rows = users.data?.data ?? [];
  const meta = users.data?.meta ?? { page, limit, total: 0, total_pages: 0 };
  const { loading, error, reload: load } = users;

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
            u.is_active
              ? { key: "deactivate", label: "Nonaktifkan", icon: <UserX />, destructive: true, onSelect: () => setToggling(u) }
              : { key: "activate", label: "Aktifkan", icon: <UserCheck />, onSelect: () => setToggling(u) },
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

      <UserFormSheet open={sheetOpen} onClose={() => setSheetOpen(false)} user={editing} onSaved={load} />
      <ConfirmDialog
        open={toggling !== undefined}
        onOpenChange={(open) => !open && setToggling(undefined)}
        tone={toggling?.is_active ? "danger" : "primary"}
        title={toggling?.is_active ? `Nonaktifkan ${toggling.name}?` : `Aktifkan ${toggling?.name ?? "pengguna"}?`}
        description={
          toggling?.is_active
            ? "Pengguna ini langsung keluar dari semua perangkat dan tidak bisa masuk lagi. Datanya tetap tersimpan, dan akun bisa diaktifkan kembali."
            : "Pengguna ini bisa masuk lagi dengan kata sandinya."
        }
        confirmLabel={toggling?.is_active ? "Nonaktifkan" : "Aktifkan"}
        onConfirm={async () => {
          if (!toggling) return;
          await setUserActive(toggling, !toggling.is_active);
          notify.success(toggling.is_active ? "Pengguna dinonaktifkan" : "Pengguna diaktifkan");
          load();
        }}
      />
    </Page>
  );
}
