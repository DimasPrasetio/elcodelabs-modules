import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Plus, Settings2, UserPlus } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { Pagination } from "@/shared/components/ui/pagination";
import { SearchInput } from "@/shared/components/ui/search-input";
import { Segmented } from "@/shared/components/ui/segmented";
import { Toolbar } from "@/shared/components/ui/toolbar";
import { useCan } from "@/shared/access";
import { useResource } from "@/shared/hooks/use-resource";
import { CodeSettingsDialog } from "@/modules/suppliers/components/CodeSettingsDialog";
import { SupplierFormSheet } from "@/modules/suppliers/components/SupplierFormSheet";
import { SupplierStatusBadge } from "@/modules/suppliers/components/SupplierStatusBadge";
import { listSuppliers } from "@/modules/suppliers/services/suppliers.service";
import { TAX_ID_LABEL, termsLabel } from "@/modules/suppliers/types/supplier";
import type { Supplier, SupplierStatus } from "@/modules/suppliers/types/supplier";

const STATUSES: { value: SupplierStatus; label: string }[] = [
  { value: "active", label: "Aktif" },
  { value: "on_hold", label: "Ditahan" },
  { value: "inactive", label: "Nonaktif" },
  { value: "", label: "Semua" },
];

export default function SuppliersPage() {
  const navigate = useNavigate();
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<SupplierStatus>("active");
  const [creating, setCreating] = useState(false);
  const [settings, setSettings] = useState(false);
  const canManage = useCan("suppliers.manage");

  const suppliers = useResource((signal) => listSuppliers({ page, limit, q: query, status }, signal), [page, limit, query, status]);
  const rows = suppliers.data?.data ?? [];
  const meta = suppliers.data?.meta ?? { page, limit, total: 0, total_pages: 0 };
  const open = (c: Supplier) => navigate(`/suppliers/${c.id}`);

  const columns: DataTableColumn<Supplier>[] = [
    { header: "Kode", width: 130, render: (c) => <span className="font-mono text-sm">{c.code}</span> },
    {
      header: "Nama",
      render: (c) => (
        <div className="min-w-0">
          <div className="truncate font-medium">{c.name}</div>
          {c.legal_name && c.legal_name !== c.name && <div className="truncate text-xs text-muted-foreground">{c.legal_name}</div>}
        </div>
      ),
    },
    {
      header: "Identitas pajak",
      render: (c) =>
        c.tax_id_kind === "none" ? (
          <span className="text-muted-foreground">–</span>
        ) : (
          <span className="text-sm">
            <span className="text-muted-foreground">{TAX_ID_LABEL[c.tax_id_kind]} </span>
            <span className="font-mono">{c.tax_id}</span>
          </span>
        ),
    },
    { header: "Termin", width: 120, render: (c) => termsLabel(c.payment_days) },
    { header: "Kontak", id: "contact", defaultHidden: true, render: (c) => <span className="text-muted-foreground">{c.phone || c.email || "–"}</span> },
    { header: "Status", width: 120, render: (c) => <SupplierStatusBadge supplier={c} /> },
  ];

  return (
    <Page>
      <PageHeader
        title="Pemasok"
        description="Pihak yang menjual kepada perusahaan ini. Pemasok yang sudah dipakai di dokumen dinonaktifkan, tidak dihapus."
        actions={
          canManage && (
            <>
              <Button variant="outline" onClick={() => setSettings(true)}>
                <Settings2 />
                Kode otomatis
              </Button>
              <Button onClick={() => setCreating(true)}>
                <Plus />
                Tambah pemasok
              </Button>
            </>
          )
        }
      />
      <section className="overflow-hidden rounded-xl border border-border bg-card">
        <Toolbar
          plain
          search={
            <SearchInput
              value={query}
              placeholder="Cari kode, nama, atau NPWP…"
              onSearch={(q) => {
                setPage(1);
                setQuery(q);
              }}
            />
          }
          primary={
            <Segmented
              aria-label="Status"
              options={STATUSES}
              value={status}
              onChange={(s) => {
                setPage(1);
                setStatus(s);
              }}
            />
          }
          active={query ? [{ key: "q", label: `Cari: ${query}`, onRemove: () => setQuery("") }] : []}
        />
        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(c) => c.id}
          loading={suppliers.loading}
          error={suppliers.error}
          onRetry={suppliers.reload}
          onRowClick={open}
          rowLabel={(c) => `Buka ${c.name}`}
          caption="Daftar pemasok"
          emptyTitle={query ? "Tidak ada yang cocok" : "Belum ada pemasok"}
          emptyDescription={query ? `Tidak ada pemasok untuk "${query}".` : "Tambahkan pemasok sebelum membuat dokumen pembelian."}
          emptyAction={
            query ? (
              <Button variant="outline" size="sm" onClick={() => setQuery("")}>
                Hapus pencarian
              </Button>
            ) : (
              canManage &&
              status === "active" && (
                <Button size="sm" onClick={() => setCreating(true)}>
                  <UserPlus />
                  Tambah pemasok pertama
                </Button>
              )
            )
          }
        />
        {!suppliers.error && rows.length > 0 && (
          <Pagination
            page={meta.page}
            limit={meta.limit}
            total={meta.total}
            noun="pemasok"
            onPageChange={setPage}
            onLimitChange={(n) => {
              setPage(1);
              setLimit(n);
            }}
          />
        )}
      </section>
      <SupplierFormSheet open={creating} onClose={() => setCreating(false)} onSaved={(c) => navigate(`/suppliers/${c.id}`)} />
      <CodeSettingsDialog open={settings} onClose={() => setSettings(false)} />
    </Page>
  );
}
