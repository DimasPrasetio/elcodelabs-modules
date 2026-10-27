import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";
import { NativeSelect } from "@/shared/components/primitives/native-select";
import { Switch } from "@/shared/components/primitives/switch";
import { FormField } from "@/shared/components/ui/form-field";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { resetFormat, sample, saveFormat } from "@/modules/numbering/services/numbering.service";
import type { DocType, NumberFormat, Reset } from "@/modules/numbering/types/numbering";

export const RESETS: Record<Reset, string> = { never: "Tidak pernah", yearly: "Tiap tahun", monthly: "Tiap bulan", daily: "Tiap hari" };

const TOKENS: [string, string][] = [
  ["{YYYY}", "tahun, 2026"],
  ["{YY}", "tahun, 26"],
  ["{MM}", "bulan, 10"],
  ["{DD}", "tanggal, 27"],
  ["{SEQ:4}", "urutan, 0007 (wajib)"],
  ["{BRANCH}", "kode cabang"],
  ["{COMPANY}", "kode perusahaan"],
];

interface FormatDialogProps {
  type: DocType | null;
  onClose: () => void;
  onSaved: () => void;
}

/** Edits a document type's number format, with a sample of the next number as you type. */
export function FormatDialog({ type, onClose, onSaved }: FormatDialogProps) {
  const [values, setValues] = useState<NumberFormat>({ pattern: "", reset: "never", per_branch: false });
  const [preview, setPreview] = useState<{ sample?: string; error?: string }>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!type) return;
    setValues(type.format);
    setFormError(null);
  }, [type]);

  useEffect(() => {
    if (!type) return;
    const ctrl = new AbortController();
    const timer = setTimeout(() => {
      sample(values, ctrl.signal)
        .then((s) => setPreview({ sample: s }))
        .catch((err) => {
          if (ctrl.signal.aborted) return;
          setPreview({ error: apiFieldErrors(err).pattern ?? apiErrorMessage(err) });
        });
    }, 300);
    return () => {
      clearTimeout(timer);
      ctrl.abort();
    };
  }, [type, values]);

  async function run(action: () => Promise<void>, message: string) {
    setSaving(true);
    setFormError(null);
    try {
      await action();
      notify.success(message);
      onSaved();
      onClose();
    } catch (err) {
      setFormError(apiFieldErrors(err).pattern ?? apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (type) void run(() => saveFormat(type.doc_type, values), "Format nomor disimpan");
  }

  return (
    <Modal
      open={type !== null}
      onClose={() => !saving && onClose()}
      title={`Format nomor — ${type?.label ?? ""}`}
      description="Nomor yang sudah terpakai tidak berubah. Mengganti cara mulai ulang atau per cabang memulai urutan baru."
      actions={
        <>
          {type?.custom && (
            <Button variant="ghost" className="mr-auto" disabled={saving} onClick={() => run(() => resetFormat(type.doc_type), "Format dikembalikan ke bawaan")}>
              Kembalikan ke bawaan
            </Button>
          )}
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button type="submit" form="format-form" loading={saving} disabled={Boolean(preview.error)}>
            Simpan
          </Button>
        </>
      }
    >
      <form id="format-form" onSubmit={submit} noValidate className="grid gap-4">
        {formError && <Notice tone="danger" title="Gagal menyimpan">{formError}</Notice>}
        <FormField label="Pola" required error={preview.error} hint={preview.sample ? `Contoh: ${preview.sample}` : undefined}>
          <Input value={values.pattern} onChange={(e) => setValues((v) => ({ ...v, pattern: e.target.value }))} maxLength={60} className="font-mono" spellCheck={false} />
        </FormField>
        <ul className="grid grid-cols-2 gap-x-4 gap-y-1 text-xs text-muted-foreground">
          {TOKENS.map(([token, meaning]) => (
            <li key={token}>
              <code className="font-mono text-foreground">{token}</code> {meaning}
            </li>
          ))}
        </ul>
        <div className="grid gap-4 sm:grid-cols-2">
          <FormField label="Mulai ulang dari 1">
            <NativeSelect value={values.reset} onChange={(e) => setValues((v) => ({ ...v, reset: e.target.value as Reset }))}>
              {Object.entries(RESETS).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </NativeSelect>
          </FormField>
          <FormField label="Urutan per cabang">
            <div className="flex h-9 items-center gap-2 text-sm">
              <Switch checked={values.per_branch} onCheckedChange={(on) => setValues((v) => ({ ...v, per_branch: on }))} aria-label="Urutan per cabang" />
              {values.per_branch ? "Tiap cabang punya urutan sendiri" : "Satu urutan untuk perusahaan"}
            </div>
          </FormField>
        </div>
      </form>
    </Modal>
  );
}
