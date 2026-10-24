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
import { createRole, updateRole } from "@/modules/rbac/services/rbac.service";
import type { Role } from "@/modules/rbac/types/rbac";

interface RoleFormDialogProps {
  open: boolean;
  onClose: () => void;
  /** The role to rename; absent to create one (permissions are chosen on its page). */
  role?: Role;
  onSaved: (role: Role) => void;
}

/** A role's name and description — two fields, so a dialog. */
export function RoleFormDialog({ open, onClose, role, onSaved }: RoleFormDialogProps) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setName(role?.name ?? "");
    setDescription(role?.description ?? "");
    setErrors({});
    setFormError(null);
  }, [open, role]);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!name.trim()) {
      setErrors({ name: "Nama wajib diisi" });
      return;
    }
    setSaving(true);
    setErrors({});
    setFormError(null);
    try {
      const values = { name, description, permissions: role?.permissions ?? [] };
      const saved = role ? await updateRole(role.id, values) : await createRole(values);
      notify.success(role ? "Peran diperbarui" : "Peran dibuat");
      onSaved(saved);
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
      title={role ? "Ubah peran" : "Peran baru"}
      description={role ? "Ganti nama atau keterangan peran." : "Izinnya dipilih setelah ini, di halaman peran."}
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button type="submit" form="role-form" loading={saving}>
            {role ? "Simpan" : "Buat peran"}
          </Button>
        </>
      }
    >
      <form id="role-form" onSubmit={submit} noValidate className="grid gap-4">
        {formError && <Notice tone="danger" title="Gagal menyimpan">{formError}</Notice>}
        <FormField label="Nama" required error={errors.name}>
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="mis. Staf gudang" />
        </FormField>
        <FormField label="Keterangan" error={errors.description} hint="Opsional — untuk siapa peran ini.">
          <Textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={3} />
        </FormField>
      </form>
    </Modal>
  );
}
