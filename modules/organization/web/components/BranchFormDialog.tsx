import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";
import { Textarea } from "@/shared/components/primitives/textarea";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { FormField } from "@/shared/components/ui/form-field";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { notify } from "@/shared/lib/notify";
import { ZoneSelect } from "@/modules/organization/components/ZoneSelect";
import { createBranch, updateBranch } from "@/modules/organization/services/organization.service";
import type { Branch, BranchValues } from "@/modules/organization/types/organization";

interface BranchFormDialogProps {
  open: boolean;
  onClose: () => void;
  /** The branch to change; absent to create one in the active company. */
  branch?: Branch;
  onSaved: () => void;
}

const EMPTY: BranchValues = { code: "", name: "", timezone: "", nitku: "", address: "" };

export function BranchFormDialog({ open, onClose, branch, onSaved }: BranchFormDialogProps) {
  const [values, setValues] = useState<BranchValues>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setValues(branch ? { code: branch.code, name: branch.name, timezone: branch.timezone, nitku: branch.nitku, address: branch.address } : EMPTY);
    setErrors({});
    setFormError(null);
  }, [open, branch]);

  const set = (k: keyof BranchValues) => (v: string) => setValues((s) => ({ ...s, [k]: v }));

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setErrors({});
    setFormError(null);
    try {
      if (branch) await updateBranch(branch.id, values);
      else await createBranch(values);
      notify.success(branch ? "Cabang diperbarui" : "Cabang dibuat");
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
      title={branch ? "Ubah cabang" : "Cabang baru"}
      description={branch ? undefined : "Tempat kegiatan usaha perusahaan ini — bukan badan hukum tersendiri."}
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button type="submit" form="branch-form" loading={saving}>
            {branch ? "Simpan" : "Buat cabang"}
          </Button>
        </>
      }
    >
      <form id="branch-form" onSubmit={submit} noValidate className="grid gap-4">
        {formError && <Notice tone="danger" title="Gagal menyimpan">{formError}</Notice>}
        <div className="grid gap-4 sm:grid-cols-2">
          <FormField label="Kode" required error={errors.code} hint="mis. BDG">
            <Input value={values.code} onChange={(e) => set("code")(e.target.value.toUpperCase())} maxLength={20} />
          </FormField>
          <FormField label="Nama" required error={errors.name}>
            <Input value={values.name} onChange={(e) => set("name")(e.target.value)} maxLength={120} placeholder="mis. Cabang Bandung" />
          </FormField>
        </div>
        <FormField label="Zona waktu" error={errors.timezone} hint="Cabang di WITA atau WIT memulai harinya lebih awal dari kantor pusat di WIB.">
          <ZoneSelect value={values.timezone} onChange={set("timezone")} inherit />
        </FormField>
        <FormField label="NITKU" error={errors.nitku} hint="22 angka: NPWP pusat 16 digit + nomor urut tempat usaha 6 digit. Opsional.">
          <Input value={values.nitku} onChange={(e) => set("nitku")(e.target.value.replace(/\D/g, ""))} inputMode="numeric" maxLength={22} />
        </FormField>
        <FormField label="Alamat" error={errors.address}>
          <Textarea value={values.address} onChange={(e) => set("address")(e.target.value)} rows={2} maxLength={255} />
        </FormField>
      </form>
    </Modal>
  );
}
