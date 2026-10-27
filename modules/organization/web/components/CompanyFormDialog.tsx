import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { FormField } from "@/shared/components/ui/form-field";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { notify } from "@/shared/lib/notify";
import { ZoneSelect } from "@/modules/organization/components/ZoneSelect";
import { createCompany, updateCompany } from "@/modules/organization/services/organization.service";
import type { Company } from "@/modules/organization/types/organization";

interface CompanyFormDialogProps {
  open: boolean;
  onClose: () => void;
  /** The company to change; absent to create one. */
  company?: Company;
  onSaved: () => void;
}

export function CompanyFormDialog({ open, onClose, company, onSaved }: CompanyFormDialogProps) {
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [timezone, setTimezone] = useState("Asia/Jakarta");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setCode(company?.code ?? "");
    setName(company?.name ?? "");
    setTimezone(company?.timezone ?? "Asia/Jakarta");
    setErrors({});
    setFormError(null);
  }, [open, company]);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setErrors({});
    setFormError(null);
    try {
      const values = { code, name, timezone };
      if (company) await updateCompany(company.id, values);
      else await createCompany(values);
      notify.success(company ? "Perusahaan diperbarui" : "Perusahaan dibuat — Anda ditempatkan di sana sebagai Administrator");
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
      open={open}
      onClose={() => !saving && onClose()}
      size="sm"
      title={company ? "Ubah perusahaan" : "Perusahaan baru"}
      description={company ? undefined : "Badan hukum yang dilayani aplikasi ini. Nama resmi, NPWP, dan logonya diatur di Pengaturan › Umum setelah Anda pindah ke perusahaan itu."}
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button type="submit" form="company-form" loading={saving}>
            {company ? "Simpan" : "Buat perusahaan"}
          </Button>
        </>
      }
    >
      <form id="company-form" onSubmit={submit} noValidate className="grid gap-4">
        {formError && <Notice tone="danger" title="Gagal menyimpan">{formError}</Notice>}
        <FormField label="Kode" required error={errors.code} hint="Singkat, huruf besar dan angka — mis. PT-MAJU.">
          <Input value={code} onChange={(e) => setCode(e.target.value.toUpperCase())} maxLength={20} />
        </FormField>
        <FormField label="Nama singkat" required error={errors.name} hint="Tampil di pemilih perusahaan.">
          <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={120} />
        </FormField>
        <FormField label="Zona waktu" required error={errors.timezone} hint="Menentukan batas hari untuk tanggal dokumen dan laporan.">
          <ZoneSelect value={timezone} onChange={setTimezone} />
        </FormField>
      </form>
    </Modal>
  );
}
