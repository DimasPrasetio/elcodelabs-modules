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
import { CodeSettingsDialog } from "@/modules/customers/components/CodeSettingsDialog";
import { CustomerFormSheet } from "@/modules/customers/components/CustomerFormSheet";
import { CustomerStatusBadge } from "@/modules/customers/components/CustomerStatusBadge";
import { listCustomers } from "@/modules/customers/services/customers.service";
import { TAX_ID_LABEL, termsLabel } from "@/modules/customers/types/customer";
import type { Customer, CustomerStatus } from "@/modules/customers/types/customer";

const STATUSES: { value: CustomerStatus; label: string }[] = [
  { value: "active", label: "Aktif" },
  { value: "on_hold", label: "Ditahan" },
  { value: "inactive", label: "Nonaktif" },
  { value: "", label: "Semua" },
];

export default function CustomersPage() {
  const navigate = useNavigate();
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<CustomerStatus>("active");
  const [creating, setCreating] = useState(false);
  const [settings, setSettings] = useState(false);
  const canManage = useCan("customers.manage");

  const customers = useResource((signal) => listCustomers({ page, limit, q: query, status }, signal), [page, limit, query, status]);
  const rows = customers.data?.data ?? [];
  const meta = customers.data?.meta ?? { page, limit, total: 0, total_pages: 0 };
  const open = (c: Customer) => navigate(`/customers/${c.id}`);

  const columns: DataTableColumn<Customer>[] = [
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
    { header: "Status", width: 120, render: (c) => <CustomerStatusBadge customer={c} /> },
  ];

  return (
    <Page>
      <PageHeader
        title="Pelanggan"
        description="Pihak yang membeli dari perusahaan ini. Pelanggan yang sudah dipakai di dokumen dinonaktifkan, tidak dihapus."
        actions={
          canManage && (
            <>
              <Button variant="outline" onClick={() => setSettings(true)}>
                <Settings2 />
                Kode otomatis
              </Button>
              <Button onClick={() => setCreating(true)}>
                <Plus />
                Tambah pelanggan
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
          loading={customers.loading}
          error={customers.error}
          onRetry={customers.reload}
          onRowClick={open}
          rowLabel={(c) => `Buka ${c.name}`}
          caption="Daftar pelanggan"
          emptyTitle={query ? "Tidak ada yang cocok" : "Belum ada pelanggan"}
          emptyDescription={query ? `Tidak ada pelanggan untuk "${query}".` : "Tambahkan pelanggan sebelum membuat dokumen penjualan."}
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
                  Tambah pelanggan pertama
                </Button>
              )
            )
          }
        />
        {!customers.error && rows.length > 0 && (
          <Pagination
            page={meta.page}
            limit={meta.limit}
            total={meta.total}
            noun="pelanggan"
            onPageChange={setPage}
            onLimitChange={(n) => {
              setPage(1);
              setLimit(n);
            }}
          />
        )}
      </section>
      <CustomerFormSheet open={creating} onClose={() => setCreating(false)} onSaved={(c) => navigate(`/customers/${c.id}`)} />
      <CodeSettingsDialog open={settings} onClose={() => setSettings(false)} />
    </Page>
  );
}
