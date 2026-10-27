import { useState } from "react";
import { Button } from "@/shared/components/primitives/button";
import { EmptyState } from "@/shared/components/ui/empty-state";
import { ErrorState } from "@/shared/components/ui/error-state";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { SectionCard } from "@/shared/components/ui/section-card";
import { DetailSkeleton } from "@/shared/components/ui/skeletons";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import { useResource } from "@/shared/hooks/use-resource";
import { FormatDialog, RESETS } from "@/modules/numbering/components/FormatDialog";
import { NextNumberDialog } from "@/modules/numbering/components/NextNumberDialog";
import { fetchOverview } from "@/modules/numbering/services/numbering.service";
import type { DocType } from "@/modules/numbering/types/numbering";

export default function NumberingPage() {
  const overview = useResource((signal) => fetchOverview(signal), []);
  const [editing, setEditing] = useState<DocType | null>(null);
  const [raising, setRaising] = useState<DocType | null>(null);

  if (overview.error) return <ErrorState message={overview.error} onRetry={overview.reload} />;
  if (!overview.data) return <DetailSkeleton />;
  const { types, manage_formats: canFormat } = overview.data;

  return (
    <Page>
      <PageHeader
        title="Penomoran"
        description="Nomor dokumen diberikan saat dokumen diposting, berurutan tanpa lompatan. Modul yang terpasang menentukan jenis dokumennya."
      />
      {types.length === 0 && (
        <EmptyState title="Belum ada jenis dokumen" description="Jenis dokumen muncul di sini setelah modul yang memakainya (penjualan, pembelian, …) dipasang." />
      )}
      {types.map((t) => (
        <SectionCard
          key={t.doc_type}
          title={t.label}
          description={
            <span className="flex flex-wrap items-center gap-2">
              <code className="font-mono text-foreground">{t.format.pattern}</code>
              <span>· mulai ulang {RESETS[t.format.reset].toLowerCase()}</span>
              {t.format.per_branch && <span>· per cabang</span>}
              {t.custom ? <StatusBadge tone="info">Diubah</StatusBadge> : <StatusBadge tone="neutral">Bawaan</StatusBadge>}
            </span>
          }
          actions={
            <>
              {t.series.some((s) => s.manageable) && (
                <Button size="sm" variant="outline" onClick={() => setRaising(t)}>
                  Nomor berikutnya
                </Button>
              )}
              {canFormat && (
                <Button size="sm" variant="outline" onClick={() => setEditing(t)}>
                  Ubah format
                </Button>
              )}
            </>
          }
          flush
        >
          <ul className="divide-y divide-border">
            {t.series.map((s) => (
              <li key={s.branch_id} className="flex items-center justify-between gap-3 px-5 py-2.5 text-sm">
                <span className="text-muted-foreground">{s.branch_id === 0 ? "Seluruh perusahaan" : s.branch_name || `Cabang #${s.branch_id}`}</span>
                <span>
                  <span className="mr-2 text-xs text-muted-foreground">berikutnya</span>
                  <code className="font-mono font-medium">{s.next}</code>
                </span>
              </li>
            ))}
          </ul>
        </SectionCard>
      ))}
      <FormatDialog type={editing} onClose={() => setEditing(null)} onSaved={overview.reload} />
      <NextNumberDialog type={raising} onClose={() => setRaising(null)} onSaved={overview.reload} />
    </Page>
  );
}
