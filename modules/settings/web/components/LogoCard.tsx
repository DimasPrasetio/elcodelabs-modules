import { useState } from "react";
import { Trash2 } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { FileDropzone } from "@/shared/components/ui/file-dropzone";
import { SectionCard } from "@/shared/components/ui/section-card";
import { notify } from "@/shared/lib/notify";
import { logoUrl, removeLogo, uploadLogo } from "@/modules/settings/services/settings.service";
import type { Company } from "@/modules/settings/types/settings";

/** The company logo: preview, replace, remove. Shown only when a file store is wired. */
export function LogoCard({ company, onChange }: { company: Company; onChange: (c: Company) => void }) {
  const [busy, setBusy] = useState(false);
  const [version, setVersion] = useState(() => Date.now());

  async function run(action: () => Promise<Company>, done: string) {
    setBusy(true);
    try {
      onChange(await action());
      setVersion(Date.now());
      notify.success(done);
    } catch (err) {
      notify.error(err);
    } finally {
      setBusy(false);
    }
  }

  return (
    <SectionCard
      title="Logo"
      description="Tampil di kop dokumen dan email. PNG, JPEG, atau WebP, paling besar 2 MB."
      actions={
        company.has_logo && (
          <Button variant="outline" size="sm" disabled={busy} onClick={() => void run(removeLogo, "Logo dihapus")}>
            <Trash2 />
            Hapus
          </Button>
        )
      }
    >
      <div className="grid gap-4 sm:grid-cols-[160px_1fr] sm:items-center">
        <div className="flex aspect-square items-center justify-center overflow-hidden rounded-lg border border-border bg-background">
          {company.has_logo ? (
            <img src={logoUrl(version)} alt="Logo perusahaan" className="max-h-full max-w-full object-contain p-3" />
          ) : (
            <span className="px-3 text-center text-xs text-muted-foreground">Belum ada logo</span>
          )}
        </div>
        <FileDropzone
          accept="image/png,image/jpeg,image/webp"
          busy={busy}
          label={company.has_logo ? "Ganti logo: seret gambar ke sini atau pilih" : "Seret gambar ke sini atau pilih"}
          hint="PNG, JPEG, atau WebP · maks. 2 MB"
          onFile={(f) => void run(() => uploadLogo(f), "Logo disimpan")}
        />
      </div>
    </SectionCard>
  );
}
