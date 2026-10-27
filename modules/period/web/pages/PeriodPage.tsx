import { useEffect, useState } from "react";
import { Lock, LockOpen } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { NativeSelect } from "@/shared/components/primitives/native-select";
import { FormField } from "@/shared/components/ui/form-field";
import { Notice } from "@/shared/components/ui/notice";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { SectionCard } from "@/shared/components/ui/section-card";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import { DetailSkeleton } from "@/shared/components/ui/skeletons";
import { ErrorState } from "@/shared/components/ui/error-state";
import { useResource } from "@/shared/hooks/use-resource";
import { formatDate } from "@/shared/lib/format";
import { notify } from "@/shared/lib/notify";
import { LockDialog } from "@/modules/period/components/LockDialog";
import { fetchOverview, setFiscalYearStart } from "@/modules/period/services/period.service";
import type { AreaLock } from "@/modules/period/types/period";

const MONTHS = ["Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"];

/** The fiscal year today falls in, as "1 Jul 2025 – 30 Jun 2026". */
function fiscalYear(today: string, startMonth: number): string {
  const [y, m] = today.split("-").map(Number);
  const startYear = m >= startMonth ? y : y - 1;
  const start = new Date(Date.UTC(startYear, startMonth - 1, 1));
  const end = new Date(Date.UTC(startYear + 1, startMonth - 1, 0));
  const iso = (d: Date) => d.toISOString().slice(0, 10);
  return `${formatDate(iso(start))} – ${formatDate(iso(end))}`;
}

export default function PeriodPage() {
  const overview = useResource((signal) => fetchOverview(signal), []);
  const [editing, setEditing] = useState<AreaLock | null>(null);
  const [month, setMonth] = useState(1);
  const [savingMonth, setSavingMonth] = useState(false);

  useEffect(() => {
    if (overview.data) setMonth(overview.data.fiscal_year_start_month);
  }, [overview.data]);

  if (overview.error) return <ErrorState message={overview.error} onRetry={overview.reload} />;
  if (!overview.data) return <DetailSkeleton />;
  const data = overview.data;
  // Locks belong to the whole company: the server says whether this user reaches all of it.
  const canClose = data.can_close;
  const canReopen = data.can_reopen;
  const allThrough = data.areas.find((a) => a.area === "*")?.own?.locked_through ?? null;

  async function saveMonth() {
    setSavingMonth(true);
    try {
      await setFiscalYearStart(month);
      notify.success("Tahun buku disimpan");
      overview.reload();
    } catch (err) {
      notify.error(err);
    } finally {
      setSavingMonth(false);
    }
  }

  return (
    <Page>
      <PageHeader
        title="Tutup buku"
        description="Dokumen bertanggal pada atau sebelum tanggal kunci tidak bisa diposting atau dibatalkan, jadi laporan yang sudah diserahkan tidak berubah."
      />
      <SectionCard title="Tanggal kunci" description="Kunci semua area berlaku untuk setiap area; area boleh ditutup lebih jauh sendiri." flush>
        <ul className="divide-y divide-border">
          {data.areas.map((a) => (
            <li key={a.area} className="flex flex-wrap items-center justify-between gap-3 px-5 py-3">
              <div className="min-w-0">
                <div className="flex items-center gap-2 text-sm font-medium">
                  {a.label}
                  {!a.declared && <StatusBadge tone="muted">Tidak dipakai</StatusBadge>}
                </div>
                <div className="text-xs text-muted-foreground">
                  {a.own ? `Kunci sendiri sampai ${formatDate(a.own.locked_through)}` : a.area === "*" ? "Belum ada kunci" : "Tanpa kunci sendiri"}
                  {a.effective && a.own?.locked_through !== a.effective && ` · tertutup sampai ${formatDate(a.effective)} oleh kunci semua area`}
                </div>
              </div>
              <div className="flex items-center gap-3">
                {a.effective ? (
                  <StatusBadge tone="warning">
                    <Lock className="size-3" aria-hidden />
                    Ditutup s.d. {formatDate(a.effective)}
                  </StatusBadge>
                ) : (
                  <StatusBadge tone="success">
                    <LockOpen className="size-3" aria-hidden />
                    Terbuka
                  </StatusBadge>
                )}
                {/* An area no module uses any more can only lose its lock — a reopening. */}
                {(a.declared ? canClose || canReopen : Boolean(a.own) && canReopen) && (
                  <Button size="sm" variant="outline" onClick={() => setEditing(a)}>
                    Ubah
                  </Button>
                )}
              </div>
            </li>
          ))}
        </ul>
      </SectionCard>
      <SectionCard title="Tahun buku" description={`Tahun buku berjalan: ${fiscalYear(data.today, data.fiscal_year_start_month)}.`}>
        <div className="flex flex-wrap items-end gap-3">
          <div className="w-56">
            <FormField label="Bulan awal tahun buku">
              <NativeSelect value={month} onChange={(e) => setMonth(Number(e.target.value))} disabled={!canClose}>
              {MONTHS.map((name, i) => (
                <option key={name} value={i + 1}>
                  {name}
                </option>
              ))}
              </NativeSelect>
            </FormField>
          </div>
          {canClose && (
            <Button onClick={saveMonth} loading={savingMonth} disabled={month === data.fiscal_year_start_month}>
              Simpan
            </Button>
          )}
        </div>
      </SectionCard>
      <Notice tone="info" title="Riwayat perubahan">
        Setiap penutupan dan pembukaan kembali tercatat di log audit, beserta siapa, kapan, dan alasannya.
      </Notice>
      <LockDialog
        area={editing}
        allThrough={allThrough}
        today={data.today}
        canClose={canClose}
        canReopen={canReopen}
        onClose={() => setEditing(null)}
        onSaved={overview.reload}
      />
    </Page>
  );
}
