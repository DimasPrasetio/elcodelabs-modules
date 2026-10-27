import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import { Input } from "@/shared/components/primitives/input";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { AddressFields } from "@/shared/components/ui/address-fields";
import type { AddressValue } from "@/shared/components/ui/address-fields";
import { DigitsInput } from "@/shared/components/ui/digits-input";
import { FormField } from "@/shared/components/ui/form-field";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { saveAddress } from "@/modules/customers/services/customers.service";
import { COUNTRIES } from "@/modules/customers/types/customer";
import type { Address, AddressValues } from "@/modules/customers/types/customer";

interface AddressDialogProps {
  open: boolean;
  onClose: () => void;
  customerId: number;
  /** Whether the customer has an NPWP or NIK, which a NITKU must start with. */
  hasTaxID: boolean;
  /** The address to change; absent to add one. */
  address?: Address;
  /** The first address is the default billing and shipping one. */
  first: boolean;
  onSaved: () => void;
}

const EMPTY: AddressValues = {
  label: "",
  line: "",
  city: "",
  province: "",
  postal_code: "",
  country: "ID",
  nitku: "",
  for_billing: true,
  for_shipping: true,
  is_default_billing: false,
  is_default_shipping: false,
};

export function AddressDialog({ open, onClose, customerId, hasTaxID, address, first, onSaved }: AddressDialogProps) {
  const [values, setValues] = useState<AddressValues>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setValues(address ? { ...address } : { ...EMPTY, label: first ? "Kantor pusat" : "", is_default_billing: first, is_default_shipping: first });
    setErrors({});
    setFormError(null);
  }, [open, address, first]);

  const set = <K extends keyof AddressValues>(k: K) => (v: AddressValues[K]) => setValues((s) => ({ ...s, [k]: v }));
  const postal: AddressValue = { line: values.line, city: values.city, province: values.province, postal_code: values.postal_code, country: values.country };

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setErrors({});
    setFormError(null);
    try {
      await saveAddress(customerId, values, address?.id);
      notify.success("Alamat disimpan");
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
      title={address ? "Ubah alamat" : "Alamat baru"}
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button type="submit" form="customer-address-form" loading={saving}>
            Simpan
          </Button>
        </>
      }
    >
      <form id="customer-address-form" onSubmit={submit} noValidate className="grid gap-4">
        {formError && <Notice tone="danger" title="Gagal menyimpan">{formError}</Notice>}
        <FormField label="Nama alamat" required error={errors.label}>
          <Input value={values.label} onChange={(e) => set("label")(e.target.value)} maxLength={60} placeholder="mis. Kantor pusat, Gudang Cikarang" />
        </FormField>
        <AddressFields
          value={postal}
          onChange={(a) => setValues((s) => ({ ...s, ...a }))}
          errors={errors}
          countries={COUNTRIES}
        />
        {hasTaxID && (
          <FormField label="NITKU" error={errors.nitku} hint="22 angka: NPWP pelanggan + nomor urut tempat usaha 6 digit (000000 kantor pusat). Opsional.">
            <DigitsInput value={values.nitku} onChange={set("nitku")} maxDigits={22} groups={[16, 6]} />
          </FormField>
        )}
        <div className="grid gap-2 text-sm sm:grid-cols-2">
          <label className="flex items-center gap-2">
            <Checkbox checked={values.for_billing} onCheckedChange={(v) => setValues((s) => ({ ...s, for_billing: v === true, is_default_billing: v === true && s.is_default_billing }))} />
            Untuk penagihan
          </label>
          <label className="flex items-center gap-2">
            <Checkbox checked={values.for_shipping} onCheckedChange={(v) => setValues((s) => ({ ...s, for_shipping: v === true, is_default_shipping: v === true && s.is_default_shipping }))} />
            Untuk pengiriman
          </label>
          <label className="flex items-center gap-2">
            <Checkbox checked={values.is_default_billing} disabled={!values.for_billing} onCheckedChange={(v) => set("is_default_billing")(v === true)} />
            Alamat tagih utama
          </label>
          <label className="flex items-center gap-2">
            <Checkbox checked={values.is_default_shipping} disabled={!values.for_shipping} onCheckedChange={(v) => set("is_default_shipping")(v === true)} />
            Alamat kirim utama
          </label>
        </div>
        {errors.for_billing && <p className="text-xs font-medium text-destructive">{errors.for_billing}</p>}
      </form>
    </Modal>
  );
}
