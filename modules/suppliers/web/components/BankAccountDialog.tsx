import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import { Input } from "@/shared/components/primitives/input";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { FieldGrid } from "@/shared/components/ui/form-layout";
import { FormField } from "@/shared/components/ui/form-field";
import { SearchSelect } from "@/shared/components/ui/search-select";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { saveBankAccount } from "@/modules/suppliers/services/suppliers.service";
import { BANKS } from "@/modules/suppliers/types/supplier";
import type { BankAccount, BankAccountValues } from "@/modules/suppliers/types/supplier";

interface BankAccountDialogProps {
  open: boolean;
  onClose: () => void;
  supplierId: number;
  supplierName: string;
  /** The account to change, with its whole number (from reveal); absent to add one. */
  account?: BankAccount;
  first: boolean;
  onSaved: () => void;
}

const EMPTY: BankAccountValues = { bank_name: "", bank_code: "", account_number: "", account_holder: "", is_default: false };
const OTHER = "other";
const BANK_OPTIONS = [...BANKS.map((b) => ({ value: b.code, label: b.name, description: b.code })), { value: OTHER, label: "Bank lain / luar negeri" }];

export function BankAccountDialog({ open, onClose, supplierId, supplierName, account, first, onSaved }: BankAccountDialogProps) {
  const [values, setValues] = useState<BankAccountValues>(EMPTY);
  const [pick, setPick] = useState<string>("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    const v = account ? { ...account } : { ...EMPTY, account_holder: supplierName, is_default: first };
    setValues(v);
    setPick(account ? (BANKS.some((b) => b.code === account.bank_code) ? account.bank_code : OTHER) : "");
    setErrors({});
    setFormError(null);
  }, [open, account, first, supplierName]);

  const set = <K extends keyof BankAccountValues>(k: K) => (v: BankAccountValues[K]) => setValues((s) => ({ ...s, [k]: v }));

  function choose(code: string) {
    setPick(code);
    const bank = BANKS.find((b) => b.code === code);
    setValues((s) => (bank ? { ...s, bank_name: bank.name, bank_code: bank.code } : { ...s, bank_name: "", bank_code: "" }));
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setErrors({});
    setFormError(null);
    try {
      const { warnings } = await saveBankAccount(supplierId, values, account?.id);
      notify.success("Rekening disimpan");
      warnings.forEach((w) => notify.warning(w));
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
      title={account ? "Ubah rekening" : "Rekening baru"}
      description="Ke mana pembayaran untuk pemasok ini dikirim. Setiap perubahan tercatat di jejak audit."
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button type="submit" form="supplier-bank-form" loading={saving}>
            Simpan
          </Button>
        </>
      }
    >
      <form id="supplier-bank-form" onSubmit={submit} noValidate className="grid gap-4">
        {formError && <Notice tone="danger" title="Gagal menyimpan">{formError}</Notice>}
        <FormField label="Bank" required error={pick === OTHER ? undefined : errors.bank_name}>
          <SearchSelect options={BANK_OPTIONS} value={pick} onChange={(v) => choose(v == null ? "" : String(v))} />
        </FormField>
        {pick === OTHER && (
          <FieldGrid columns={2}>
            <FormField label="Nama bank" required error={errors.bank_name}>
              <Input value={values.bank_name} onChange={(e) => set("bank_name")(e.target.value)} maxLength={100} />
            </FormField>
            <FormField label="Kode bank / SWIFT" error={errors.bank_code} hint="3 angka, atau SWIFT/BIC 8–11 karakter.">
              <Input value={values.bank_code} onChange={(e) => set("bank_code")(e.target.value.toUpperCase())} maxLength={11} />
            </FormField>
          </FieldGrid>
        )}
        <FieldGrid columns={2}>
          <FormField label="Nomor rekening" required error={errors.account_number}>
            <Input value={values.account_number} onChange={(e) => set("account_number")(e.target.value.toUpperCase())} maxLength={40} className="font-mono" />
          </FormField>
          <FormField label="Atas nama" required error={errors.account_holder} hint="Seperti tercetak di buku rekening.">
            <Input value={values.account_holder} onChange={(e) => set("account_holder")(e.target.value)} maxLength={150} />
          </FormField>
        </FieldGrid>
        <label className="flex items-center gap-2 text-sm">
          <Checkbox checked={values.is_default} onCheckedChange={(v) => set("is_default")(v === true)} />
          Rekening utama untuk pembayaran
        </label>
      </form>
    </Modal>
  );
}
