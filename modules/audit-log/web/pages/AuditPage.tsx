import { useState } from "react";
import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { DateRangeFields } from "@/shared/components/ui/date-range-fields";
import { DescriptionList } from "@/shared/components/ui/description-list";
import { FormField } from "@/shared/components/ui/form-field";
import { Modal } from "@/shared/components/ui/modal";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { Pagination } from "@/shared/components/ui/pagination";
import { SearchInput } from "@/shared/components/ui/search-input";
import { Toolbar } from "@/shared/components/ui/toolbar";
import { useResource } from "@/shared/hooks/use-resource";
import { formatDateTime } from "@/shared/lib/format";
import { listAudit, listModules } from "@/modules/audit-log/services/audit.service";
import type { AuditEntry } from "@/modules/audit-log/types/audit";

const actor = (e: AuditEntry) => (e.actor_id === 0 ? "Sistem" : e.actor_name || `Pengguna #${e.actor_id}`);

/** Who did what, when — read-only, newest first. */
export default function AuditPage() {
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [module, setModule] = useState("");
  const [target, setTarget] = useState("");
  const [range, setRange] = useState({ from: "", to: "" });
  const [open, setOpen] = useState<AuditEntry>();
  const modules = useResource((signal) => listModules(signal), []);
  const entries = useResource((signal) => listAudit({ page, limit, module, target, ...range }, signal), [page, limit, module, target, range.from, range.to]);
  const rows = entries.data?.data ?? [];
  const meta = entries.data?.meta ?? { page, limit, total: 0, total_pages: 0 };
  const filtered = Boolean(module || target || range.from || range.to);

  const columns: DataTableColumn<AuditEntry>[] = [
    { header: "Waktu", id: "time", render: (e) => <span className="tabular whitespace-nowrap">{formatDateTime(e.occurred_at)}</span> },
    { header: "Pelaku", render: (e) => <span className={e.actor_id === 0 ? "text-muted-foreground" : "font-medium"}>{actor(e)}</span> },
    {
      header: "Aktivitas",
      render: (e) => (
        <span className="grid">
          <span>{e.summary || e.action}</span>
          <span className="font-mono text-xs text-muted-foreground">{e.action}</span>
        </span>
      ),
    },
    { header: "Target", render: (e) => <span className="font-mono text-xs">{e.target || "–"}</span> },
    { header: "IP", render: (e) => <span className="font-mono text-xs text-muted-foreground">{e.ip || "–"}</span> },
  ];

  return (
    <Page>
      <PageHeader title="Log aktivitas" description="Siapa mengubah apa, dan kapan. Catatan tidak dapat diubah atau dihapus." />
      <section className="overflow-hidden rounded-xl border border-border bg-card">
        <Toolbar
          plain
          search={
            <SearchInput
              value={target}
              placeholder="Target persis, mis. user:12"
              onSearch={(q) => {
                setPage(1);
                setTarget(q);
              }}
            />
          }
          primary={
            <NativeSelect
              aria-label="Modul"
              value={module}
              onChange={(e) => {
                setPage(1);
                setModule(e.target.value);
              }}
            >
              <NativeSelectOption value="">Semua modul</NativeSelectOption>
              {(modules.data ?? []).map((m) => (
                <NativeSelectOption key={m} value={m}>
                  {m}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          }
          more={
            <div className="grid gap-3">
              <DateRangeFields
                from={range.from}
                to={range.to}
                onChange={(r) => {
                  setPage(1);
                  setRange(r);
                }}
              />
            </div>
          }
          moreActiveCount={(range.from ? 1 : 0) + (range.to ? 1 : 0)}
        />
        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(e) => e.id}
          loading={entries.loading}
          error={entries.error}
          onRetry={entries.reload}
          onRowClick={setOpen}
          rowLabel={(e) => `Rincian: ${e.summary || e.action}`}
          caption="Log aktivitas"
          emptyTitle={filtered ? "Tidak ada yang cocok" : "Belum ada aktivitas"}
          emptyDescription={filtered ? "Ubah filter atau rentang tanggal." : "Perubahan data akan tercatat di sini."}
        />
        {!entries.error && rows.length > 0 && (
          <Pagination
            page={meta.page}
            limit={meta.limit}
            total={meta.total}
            noun="catatan"
            onPageChange={setPage}
            onLimitChange={(n) => {
              setPage(1);
              setLimit(n);
            }}
          />
        )}
      </section>
      <Modal open={open !== undefined} onClose={() => setOpen(undefined)} title={open?.summary || open?.action || ""} description={open && formatDateTime(open.occurred_at)} size="md">
        {open && (
          <div className="grid gap-4">
            <DescriptionList
              items={[
                { label: "Pelaku", value: actor(open) },
                { label: "Aksi", value: <span className="font-mono text-xs">{open.action}</span> },
                { label: "Modul", value: open.module },
                { label: "Target", value: open.target || "–" },
                { label: "IP", value: open.ip || "–" },
                { label: "Request ID", value: <span className="font-mono text-xs">{open.request_id || "–"}</span> },
              ]}
            />
            {open.data && (
              <FormField label="Rincian perubahan">
                <pre className="max-h-72 overflow-auto rounded-md border border-border bg-background p-3 font-mono text-xs leading-relaxed">{JSON.stringify(open.data, null, 2)}</pre>
              </FormField>
            )}
          </div>
        )}
      </Modal>
    </Page>
  );
}
