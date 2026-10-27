import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";
import { NativeSelect } from "@/shared/components/primitives/native-select";
import { Textarea } from "@/shared/components/primitives/textarea";
import { FormField } from "@/shared/components/ui/form-field";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { raiseNext } from "@/modules/numbering/services/numbering.service";
import type { DocType } from "@/modules/numbering/types/numbering";

interface NextNumberDialogProps {
  type: DocType | null;
  onClose: () => void;
  onSaved: () => void;
}

const seriesName = (name: string, id: number) => (id === 0 ? "Seluruh perusahaan" : name || `Cabang #${id}`);

/** Raises the next number of one series (this period) — when moving from another system. */
export function NextNumberDialog({ type, onClose, onSaved }: NextNumberDialogProps) {
  const [branch, setBranch] = useState(0);
  const [next, setNext] = useState("");
  const [reason, setReason] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!type) return;
    setBranch(type.series.find((s) => s.manageable)?.branch_id ?? 0);
    setNext("");
    setReason("");
    setErrors({});
    setFormError(null);
  }, [type]);

  // Only the series this user may raise.
  const choices = type?.series.filter((s) => s.manageable) ?? [];
  const series = choices.find((s) => s.branch_id === branch);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!type) return;
    const n = Number(next);
    if (!/^\d{1,9}$/.test(next) || n < 1) {
      setErrors({ next: "Isi angka bulat 1 sampai 999999999" });
      return;
    }
    setSaving(true);
    setErrors({});
    setFormError(null);
    try {
      await raiseNext(type.doc_type, branch, n, reason);
      notify.success("Nomor berikutnya disimpan");
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
      open={type !== null}
      onClose={() => !saving && onClose()}
      title={`Nomor berikutnya — ${type?.label ?? ""}`}
      description="Untuk melanjutkan nomor dari sistem lama pada periode ini. Hanya bisa dinaikkan, dan tercatat di log audit."
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button type="submit" form="next-form" loading={saving}>
            Simpan
          </Button>
        </>
      }
    >
      <form id="next-form" onSubmit={submit} noValidate className="grid gap-4">
        {formError && <Notice tone="danger" title="Gagal menyimpan">{formError}</Notice>}
        {choices.length > 1 && (
          <FormField label="Urutan" error={errors.branch_id}>
            <NativeSelect value={branch} onChange={(e) => setBranch(Number(e.target.value))}>
              {choices.map((s) => (
                <option key={s.branch_id} value={s.branch_id}>
                  {seriesName(s.branch_name, s.branch_id)}
                </option>
              ))}
            </NativeSelect>
          </FormField>
        )}
        <FormField label="Nomor urut berikutnya" required error={errors.next} hint={series ? `Sekarang: ${series.next} (urutan ${series.last_value + 1})` : undefined}>
          <Input value={next} onChange={(e) => setNext(e.target.value.replace(/\D/g, ""))} inputMode="numeric" maxLength={9} placeholder={series ? String(series.last_value + 1) : undefined} />
        </FormField>
        <FormField label="Alasan" required error={errors.reason}>
          <Textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={2} maxLength={500} placeholder="mis. melanjutkan nomor faktur dari aplikasi lama" />
        </FormField>
      </form>
    </Modal>
  );
}
