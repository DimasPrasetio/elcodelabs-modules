import { useState } from "react";
import { RotateCw } from "lucide-react";
import { ActionMenu } from "@/shared/components/ui/action-menu";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { Pagination } from "@/shared/components/ui/pagination";
import { SearchInput } from "@/shared/components/ui/search-input";
import { Segmented } from "@/shared/components/ui/segmented";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import type { Tone } from "@/shared/lib/tone";
import { Toolbar } from "@/shared/components/ui/toolbar";
import { useCan } from "@/shared/access";
import { useResource } from "@/shared/hooks/use-resource";
import { formatDateTime } from "@/shared/lib/format";
import { notify } from "@/shared/lib/notify";
import { listMessages, retryMessage } from "@/modules/mailer/services/mailer.service";
import type { MailStatus, OutboxMessage } from "@/modules/mailer/types/mail";

const STATUS: Record<MailStatus, { label: string; tone: Tone }> = {
  pending: { label: "Menunggu", tone: "info" },
  sending: { label: "Mengirim", tone: "info" },
  sent: { label: "Terkirim", tone: "success" },
  failed: { label: "Gagal", tone: "danger" },
};

const FILTERS = [
  { value: "", label: "Semua" },
  { value: "pending", label: "Menunggu" },
  { value: "sent", label: "Terkirim" },
  { value: "failed", label: "Gagal" },
];

/** Every email the app sent or is about to send: what went where, and what failed. */
export default function OutboxPage() {
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("");
  const canManage = useCan("mailer.manage");
  const messages = useResource((signal) => listMessages({ page, limit, q: query, status }, signal), [page, limit, query, status]);
  const rows = messages.data?.data ?? [];
  const meta = messages.data?.meta ?? { page, limit, total: 0, total_pages: 0 };

  async function retry(m: OutboxMessage) {
    try {
      await retryMessage(m.id);
      notify.success("Email dijadwalkan untuk dikirim ulang");
      messages.reload();
    } catch (err) {
      notify.error(err);
    }
  }

  const columns: DataTableColumn<OutboxMessage>[] = [
    {
      header: "Penerima",
      render: (m) => (
        <span className="grid">
          <span className="font-medium">{m.to_name || m.to}</span>
          {m.to_name && <span className="text-xs text-muted-foreground">{m.to}</span>}
        </span>
      ),
    },
    { header: "Subjek", render: (m) => m.subject },
    {
      header: "Status",
      render: (m) => (
        <span className="grid gap-1">
          <StatusBadge tone={STATUS[m.status].tone}>{STATUS[m.status].label}</StatusBadge>
          {m.last_error && <span className="max-w-72 truncate text-xs text-muted-foreground" title={m.last_error}>{m.last_error}</span>}
        </span>
      ),
    },
    { header: "Percobaan", align: "right", width: 96, render: (m) => m.attempts },
    { header: "Dibuat", id: "created", render: (m) => <span className="whitespace-nowrap">{formatDateTime(m.created_at)}</span> },
  ];
  if (canManage) {
    columns.push({
      header: "",
      id: "actions",
      width: 56,
      align: "right",
      render: (m) =>
        m.status === "failed" ? (
          <ActionMenu variant="ghost" size="icon-sm" label={`Aksi untuk email ke ${m.to}`} items={[{ key: "retry", label: "Kirim ulang", icon: <RotateCw />, onSelect: () => void retry(m) }]} />
        ) : null,
    });
  }

  return (
    <Page>
      <PageHeader title="Kotak keluar" description="Email yang dikirim aplikasi ini. Yang gagal dicoba ulang otomatis; setelah enam kali, ditandai gagal." />
      <section className="overflow-hidden rounded-xl border border-border bg-card">
        <Toolbar
          plain
          search={
            <SearchInput
              value={query}
              placeholder="Cari penerima atau subjek…"
              onSearch={(q) => {
                setPage(1);
                setQuery(q);
              }}
            />
          }
          primary={
            <Segmented
              aria-label="Status"
              options={FILTERS}
              value={status}
              onChange={(v) => {
                setPage(1);
                setStatus(v);
              }}
            />
          }
        />
        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(m) => m.id}
          loading={messages.loading}
          error={messages.error}
          onRetry={messages.reload}
          caption="Kotak keluar"
          emptyTitle={query || status ? "Tidak ada yang cocok" : "Belum ada email"}
          emptyDescription={query || status ? "Ubah pencarian atau filter status." : "Email yang dikirim aplikasi akan muncul di sini."}
        />
        {!messages.error && rows.length > 0 && (
          <Pagination
            page={meta.page}
            limit={meta.limit}
            total={meta.total}
            noun="email"
            onPageChange={setPage}
            onLimitChange={(n) => {
              setPage(1);
              setLimit(n);
            }}
          />
        )}
      </section>
    </Page>
  );
}
