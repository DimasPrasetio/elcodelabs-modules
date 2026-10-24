import { useCallback, useEffect, useState } from "react";
import { MoreHorizontal, Plus, Search, UserPlus } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { Card } from "@/shared/components/primitives/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/shared/components/primitives/dropdown-menu";
import { Input } from "@/shared/components/primitives/input";
import { ConfirmDialog } from "@/shared/components/ui/confirm-dialog";
import { DataTable } from "@/shared/components/ui/data-table";
import type { Column } from "@/shared/components/ui/data-table";
import { EmptyState } from "@/shared/components/ui/empty-state";
import { ErrorState } from "@/shared/components/ui/error-state";
import { PageHeader } from "@/shared/components/ui/page-header";
import { PaginationBar } from "@/shared/components/ui/pagination-bar";
import { apiErrorMessage } from "@/shared/services/api-error";
import type { PageMeta } from "@/shared/types/api";
import { UserFormDialog } from "@/modules/users/components/UserFormDialog";
import { deleteUser, listUsers } from "@/modules/users/services/users.service";
import type { User } from "@/modules/users/types/user";

const LIMIT = 25;
const dateFormat = new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", year: "numeric" });

function StatusPill({ active }: { active: boolean }) {
  return active ? (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-success-soft px-2 py-0.5 text-xs font-medium text-success">
      <span className="size-1.5 rounded-full bg-current" />
      Active
    </span>
  ) : (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
      <span className="size-1.5 rounded-full bg-current" />
      Inactive
    </span>
  );
}

export default function UsersPage() {
  const [rows, setRows] = useState<User[]>([]);
  const [meta, setMeta] = useState<PageMeta>({ page: 1, limit: LIMIT, total: 0, total_pages: 0 });
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<User | undefined>();
  const [formOpen, setFormOpen] = useState(false);
  const [deleting, setDeleting] = useState<User | undefined>();

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await listUsers({ page, limit: LIMIT, q: query });
      setRows(res.data);
      setMeta(res.meta);
    } catch (err) {
      setError(apiErrorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [page, query]);

  useEffect(() => {
    void load();
  }, [load]);

  // Search as the user types, without a request per keystroke.
  useEffect(() => {
    const t = setTimeout(() => {
      setPage(1);
      setQuery(search.trim());
    }, 300);
    return () => clearTimeout(t);
  }, [search]);

  function openCreate() {
    setEditing(undefined);
    setFormOpen(true);
  }

  function openEdit(user: User) {
    setEditing(user);
    setFormOpen(true);
  }

  const columns: Column<User>[] = [
    { key: "name", header: "Name", cell: (u) => <span className="font-medium">{u.name}</span> },
    { key: "email", header: "Email", cell: (u) => <span className="text-muted-foreground">{u.email}</span> },
    { key: "status", header: "Status", cell: (u) => <StatusPill active={u.is_active} /> },
    { key: "created", header: "Added", cell: (u) => dateFormat.format(new Date(u.created_at)) },
    {
      key: "actions",
      header: <span className="sr-only">Actions</span>,
      className: "w-12",
      cell: (u) => (
        <div onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()}>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon-sm" aria-label={`Actions for ${u.name}`}>
                <MoreHorizontal />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => openEdit(u)}>Edit</DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" onSelect={() => setDeleting(u)}>
                Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      ),
    },
  ];

  const filtered = query !== "";

  return (
    <>
      <PageHeader
        title="Users"
        description="People who can sign in to this app."
        actions={
          <Button onClick={openCreate}>
            <Plus />
            Add user
          </Button>
        }
      />
      <Card className="gap-0 overflow-hidden py-0">
        <div className="border-b p-3">
          <div className="relative max-w-sm">
            <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search name or email…"
              aria-label="Search users"
              className="pl-8"
            />
          </div>
        </div>
        {error ? (
          <ErrorState message={error} onRetry={load} />
        ) : (
          <>
            <DataTable
              columns={columns}
              rows={rows}
              getRowId={(u) => u.id}
              loading={loading}
              skeletonRows={8}
              onRowClick={openEdit}
              rowLabel={(u) => `Edit ${u.name}`}
              empty={
                filtered ? (
                  <EmptyState
                    title="No users match"
                    description={`Nothing found for "${query}".`}
                    action={
                      <Button variant="outline" size="sm" onClick={() => setSearch("")}>
                        Clear search
                      </Button>
                    }
                  />
                ) : (
                  <EmptyState
                    icon={UserPlus}
                    title="No users yet"
                    description="Add the people who should be able to sign in."
                    action={
                      <Button size="sm" onClick={openCreate}>
                        <Plus />
                        Add the first user
                      </Button>
                    }
                  />
                )
              }
            />
            {!loading && rows.length > 0 && <PaginationBar meta={meta} onPageChange={setPage} />}
          </>
        )}
      </Card>

      <UserFormDialog open={formOpen} onOpenChange={setFormOpen} user={editing} onSaved={() => void load()} />
      <ConfirmDialog
        open={deleting !== undefined}
        onOpenChange={(open) => !open && setDeleting(undefined)}
        title={`Delete ${deleting?.name ?? "user"}?`}
        description="They will no longer be able to sign in. This cannot be undone."
        confirmLabel="Delete"
        destructive
        onConfirm={async () => {
          if (!deleting) return;
          await deleteUser(deleting.id);
          await load();
        }}
      />
    </>
  );
}
