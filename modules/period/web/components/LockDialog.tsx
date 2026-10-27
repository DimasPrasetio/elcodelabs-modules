import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";
import { Textarea } from "@/shared/components/primitives/textarea";
import { FormField } from "@/shared/components/ui/form-field";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { formatDate } from "@/shared/lib/format";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { setLock } from "@/modules/period/services/period.service";
import type { AreaLock } from "@/modules/period/types/period";

interface LockDialogProps {
  area: AreaLock | null;
  /** The company-wide lock (YYYY-MM-DD), the earliest an area lock may be. */
  allThrough: string | null;
  today: string;
  canClose: boolean;
  canReopen: boolean;
  onClose: () => void;
  onSaved: () => void;
}

const dayBefore = (iso: string) => {
  const d = new Date(`${iso}T12:00:00Z`);
  d.setUTCDate(d.getUTCDate() - 1);
  return d.toISOString().slice(0, 10);
};

/** Moves one area's lock date: later closes more (period.manage), earlier or none reopens (period.unlock, with a reason). */
export function LockDialog({ area, allThrough, today, canClose, canReopen, onClose, onSaved }: LockDialogProps) {
  const [through, setThrough] = useState("");
  const [reason, setReason] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const current = area?.own?.locked_through ?? "";
  useEffect(() => {
    if (!area) return;
    setThrough(area.own?.locked_through ?? "");
    setReason("");
    setErrors({});
    setFormError(null);
  }, [area]);

  const reopening = Boolean(current) && (!through || through < current);
  const closing = Boolean(through) && (!current || through > current);
  const allowed = (reopening && canReopen) || (closing && canClose);
  const isAll = area?.area === "*";
  const min = !isAll && allThrough ? allThrough : undefined;

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!area || !(reopening || closing)) return onClose();
    if (reopening && reason.trim().length < 5) {
      setErrors({ reason: "Tulis alasan membuka kembali, minimal 5 karakter" });
      return;
    }
    setSaving(true);
    setErrors({});
    setFormError(null);
    try {
      await setLock(area.area, through || null, reason.trim());
      notify.success(reopening ? "Buku dibuka kembali" : "Buku ditutup");
      onSaved();
      onClose();
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (Object.keys(fields).length) setErrors(fields);
      else setFormError(apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      open={area !== null}
      onClose={() => !saving && onClose()}
      title={`Tutup buku — ${area?.label ?? ""}`}
      description={
        isAll
          ? "Tanggal ini dan sebelumnya ditutup untuk semua area: dokumen bertanggal di dalamnya tidak bisa diposting atau dibatalkan."
          : "Tanggal ini dan sebelumnya ditutup untuk area ini saja."
      }
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button type="submit" form="lock-form" loading={saving} disabled={!allowed} variant={reopening ? "destructive" : "default"}>
            {reopening ? "Buka kembali" : "Tutup buku"}
          </Button>
        </>
      }
    >
      <form id="lock-form" onSubmit={submit} noValidate className="grid gap-4">
        {formError && <Notice tone="danger" title="Gagal menyimpan">{formError}</Notice>}
        <FormField
          label="Ditutup sampai dengan"
          error={errors.locked_through}
          hint={current ? `Sekarang: ${formatDate(current)}. Kosongkan untuk menghapus kunci.` : "Belum pernah ditutup."}
        >
          <Input type="date" value={through} min={min} max={dayBefore(today)} onChange={(e) => setThrough(e.target.value)} />
        </FormField>
        {reopening && (
          <>
            <Notice tone="warning" title="Membuka kembali periode yang sudah ditutup">
              Laporan yang sudah diserahkan bisa berubah. Perubahan ini dicatat di log audit beserta alasannya.
            </Notice>
            <FormField label="Alasan" required error={errors.reason}>
              <Textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={2} maxLength={500} placeholder="mis. faktur Oktober yang tertinggal" />
            </FormField>
          </>
        )}
        {(reopening || closing) && !allowed && (
          <Notice tone="info" title={reopening ? "Membuka kembali butuh izin period.unlock" : "Menutup buku butuh izin period.manage"} />
        )}
      </form>
    </Modal>
  );
}
