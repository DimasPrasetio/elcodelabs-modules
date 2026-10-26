import { useState } from "react";
import { Download, Trash2 } from "lucide-react";
import { ActionMenu } from "@/shared/components/ui/action-menu";
import { ConfirmDialog } from "@/shared/components/ui/confirm-dialog";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { Pagination } from "@/shared/components/ui/pagination";
import { SearchInput } from "@/shared/components/ui/search-input";
import { Toolbar } from "@/shared/components/ui/toolbar";
import { useCan } from "@/shared/access";
import { useResource } from "@/shared/hooks/use-resource";
import { formatDateTime, formatNumber } from "@/shared/lib/format";
import { notify } from "@/shared/lib/notify";
import { deleteFile, downloadUrl, listFiles } from "@/modules/files/services/files.service";
import type { StoredFile } from "@/modules/files/types/file";

function size(bytes: number) {
  if (bytes >= 1 << 20) return `${formatNumber(bytes / (1 << 20), 1)} MB`;
  if (bytes >= 1 << 10) return `${formatNumber(bytes / (1 << 10), 0)} KB`;
  return `${bytes} B`;
}

/** Every file the app stores, whatever module put it there — for looking things up and cleaning up. */
export default function FilesPage() {
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [query, setQuery] = useState("");
  const [deleting, setDeleting] = useState<StoredFile>();
  const canManage = useCan("files.manage");
  const files = useResource((signal) => listFiles({ page, limit, q: query }, signal), [page, limit, query]);
  const rows = files.data?.data ?? [];
  const meta = files.data?.meta ?? { page, limit, total: 0, total_pages: 0 };

  const columns: DataTableColumn<StoredFile>[] = [
    { header: "Nama", render: (f) => <span className="font-medium break-all">{f.name}</span> },
    { header: "Modul", render: (f) => <span className="font-mono text-xs">{f.owner}</span> },
    { header: "Jenis", render: (f) => <span className="text-muted-foreground">{f.content_type}</span> },
    { header: "Ukuran", align: "right", width: 104, render: (f) => <span className="tabular whitespace-nowrap">{size(f.size)}</span> },
    { header: "Diunggah", id: "created", render: (f) => <span className="whitespace-nowrap">{formatDateTime(f.created_at)}</span> },
    {
      header: "",
      id: "actions",
      width: 56,
      align: "right",
      render: (f) => (
        <ActionMenu
          variant="ghost"
          size="icon-sm"
          label={`Aksi untuk ${f.name}`}
          items={[
            { key: "download", label: "Unduh", icon: <Download />, onSelect: () => window.open(downloadUrl(f.id), "_blank", "noopener") },
            ...(canManage ? [{ key: "delete", label: "Hapus", icon: <Trash2 />, destructive: true, onSelect: () => setDeleting(f) }] : []),
          ]}
        />
      ),
    },
  ];

  return (
    <Page>
      <PageHeader title="Berkas" description="Semua berkas yang disimpan aplikasi. Berkas yang dihapus tetap tersimpan 30 hari sebelum dibuang." />
      <section className="overflow-hidden rounded-xl border border-border bg-card">
        <Toolbar
          plain
          search={
            <SearchInput
              value={query}
              placeholder="Cari nama berkas…"
              onSearch={(q) => {
                setPage(1);
                setQuery(q);
              }}
            />
          }
        />
        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(f) => f.id}
          loading={files.loading}
          error={files.error}
          onRetry={files.reload}
          caption="Berkas"
          emptyTitle={query ? "Tidak ada yang cocok" : "Belum ada berkas"}
        />
        {!files.error && rows.length > 0 && (
          <Pagination
            page={meta.page}
            limit={meta.limit}
            total={meta.total}
            noun="berkas"
            onPageChange={setPage}
            onLimitChange={(n) => {
              setPage(1);
              setLimit(n);
            }}
          />
        )}
      </section>
      <ConfirmDialog
        open={deleting !== undefined}
        onOpenChange={(open) => !open && setDeleting(undefined)}
        title={`Hapus ${deleting?.name ?? "berkas"}?`}
        description="Modul yang memakainya tidak dapat menampilkannya lagi. Isinya dibuang permanen setelah 30 hari."
        confirmLabel="Hapus"
        onConfirm={async () => {
          if (!deleting) return;
          await deleteFile(deleting.id);
          notify.success("Berkas dihapus");
          files.reload();
        }}
      />
    </Page>
  );
}
