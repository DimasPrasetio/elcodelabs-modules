import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import { Input } from "@/shared/components/primitives/input";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { FieldGrid } from "@/shared/components/ui/form-layout";
import { FormField } from "@/shared/components/ui/form-field";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { saveContact } from "@/modules/suppliers/services/suppliers.service";
import type { Contact, ContactValues } from "@/modules/suppliers/types/supplier";

interface ContactDialogProps {
  open: boolean;
  onClose: () => void;
  supplierId: number;
  /** The contact to change; absent to add one. */
  contact?: Contact;
  first: boolean;
  onSaved: () => void;
}

const EMPTY: ContactValues = { name: "", title: "", email: "", phone: "", is_primary: false };

export function ContactDialog({ open, onClose, supplierId, contact, first, onSaved }: ContactDialogProps) {
  const [values, setValues] = useState<ContactValues>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setValues(contact ? { ...contact } : { ...EMPTY, is_primary: first });
    setErrors({});
    setFormError(null);
  }, [open, contact, first]);

  const set = <K extends keyof ContactValues>(k: K) => (v: ContactValues[K]) => setValues((s) => ({ ...s, [k]: v }));

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setErrors({});
    setFormError(null);
    try {
      await saveContact(supplierId, values, contact?.id);
      notify.success("Kontak disimpan");
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
      title={contact ? "Ubah kontak" : "Kontak baru"}
      description="Orang yang dihubungi di pihak pemasok."
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button type="submit" form="supplier-contact-form" loading={saving}>
            Simpan
          </Button>
        </>
      }
    >
      <form id="supplier-contact-form" onSubmit={submit} noValidate className="grid gap-4">
        {formError && <Notice tone="danger" title="Gagal menyimpan">{formError}</Notice>}
        <FieldGrid columns={2}>
          <FormField label="Nama" required error={errors.name}>
            <Input value={values.name} onChange={(e) => set("name")(e.target.value)} maxLength={150} />
          </FormField>
          <FormField label="Jabatan" error={errors.title}>
            <Input value={values.title} onChange={(e) => set("title")(e.target.value)} maxLength={100} placeholder="mis. Bagian pembelian" />
          </FormField>
          <FormField label="Email" error={errors.email}>
            <Input type="email" value={values.email} onChange={(e) => set("email")(e.target.value)} maxLength={190} />
          </FormField>
          <FormField label="Telepon" error={errors.phone}>
            <Input type="tel" value={values.phone} onChange={(e) => set("phone")(e.target.value)} maxLength={30} />
          </FormField>
        </FieldGrid>
        <label className="flex items-center gap-2 text-sm">
          <Checkbox checked={values.is_primary} onCheckedChange={(v) => set("is_primary")(v === true)} />
          Kontak utama
        </label>
      </form>
    </Modal>
  );
}
