import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { FormField } from "@/shared/components/ui/form-field";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { fetchSettings, saveSettings } from "@/modules/suppliers/services/suppliers.service";
import type { SupplierSettings } from "@/modules/suppliers/types/supplier";

/** The prefix of generated supplier codes, for the whole company. */
export function CodeSettingsDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [settings, setSettings] = useState<SupplierSettings | null>(null);
  const [prefix, setPrefix] = useState("");
  const [error, setError] = useState<string>();
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setError(undefined);
    setFormError(null);
    const ctl = new AbortController();
    fetchSettings(ctl.signal)
      .then((s) => {
        setSettings(s);
        setPrefix(s.code_prefix);
      })
      .catch((err) => !ctl.signal.aborted && setFormError(apiErrorMessage(err)));
    return () => ctl.abort();
  }, [open]);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setError(undefined);
    setFormError(null);
    try {
      await saveSettings(prefix);
      notify.success("Awalan kode disimpan");
      onClose();
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (fields.code_prefix) setError(fields.code_prefix);
      else setFormError(apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  const readOnly = settings !== null && !settings.can_manage;
  // The counter goes on under a new prefix: only the prefix changes.
  const sample = settings ? prefix + settings.next_code.slice(settings.code_prefix.length) : "";

  return (
    <Modal
      open={open}
      onClose={() => !saving && onClose()}
      title="Kode pemasok otomatis"
      description="Pemasok baru tanpa kode mendapat kode berikutnya. Kode yang sudah diberikan tidak berubah."
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            {readOnly ? "Tutup" : "Batal"}
          </Button>
          {!readOnly && (
            <Button type="submit" form="supplier-code-form" loading={saving} disabled={!settings}>
              Simpan
            </Button>
          )}
        </>
      }
    >
      <form id="supplier-code-form" onSubmit={submit} noValidate className="grid gap-4">
        {formError && <Notice tone="danger" title="Gagal">{formError}</Notice>}
        {readOnly && <Notice tone="info" title="Hanya bisa dilihat">Awalan kode diubah dari seluruh perusahaan oleh pemegang izin suppliers.manage.</Notice>}
        <FormField label="Awalan" error={error} hint={settings ? `Kode berikutnya: ${sample}` : undefined}>
          <Input value={prefix} onChange={(e) => setPrefix(e.target.value.toUpperCase())} maxLength={10} disabled={readOnly || !settings} placeholder="PMS-" />
        </FormField>
      </form>
    </Modal>
  );
}
