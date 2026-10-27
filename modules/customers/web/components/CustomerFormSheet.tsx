import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import axios from "axios";
import { Button } from "@/shared/components/primitives/button";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import { Input } from "@/shared/components/primitives/input";
import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { Textarea } from "@/shared/components/primitives/textarea";
import { FormSheet } from "@/shared/components/ui/form-sheet";
import { Notice } from "@/shared/components/ui/notice";
import { DigitsInput } from "@/shared/components/ui/digits-input";
import { FieldGrid } from "@/shared/components/ui/form-layout";
import { FormField } from "@/shared/components/ui/form-field";
import { SearchSelect } from "@/shared/components/ui/search-select";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { createCustomer, updateCustomer } from "@/modules/customers/services/customers.service";
import { COLLECTOR_LABEL, COUNTRIES, PARTY_LABEL, TAX_ID_LABEL } from "@/modules/customers/types/customer";
import type { Customer, CustomerValues, Duplicate, PartyType, TaxIDKind, VATCollector } from "@/modules/customers/types/customer";

interface CustomerFormSheetProps {
  open: boolean;
  onClose: () => void;
  /** The customer to change; absent to add one. */
  customer?: Customer;
  /** May the user set payment terms and the credit limit (customers.credit across the company)? */
  canCredit?: boolean;
  onSaved: (customer: Customer) => void;
}

const EMPTY: CustomerValues = {
  code: "",
  name: "",
  legal_name: "",
  party_type: "company",
  is_foreign: false,
  country: "ID",
  tax_id_kind: "npwp",
  tax_id: "",
  is_pkp: false,
  vat_collector: "none",
  email: "",
  phone: "",
  payment_days: null,
  credit_limit: null,
  notes: "",
};

function valuesOf(c: Customer): CustomerValues {
  return {
    code: c.code,
    name: c.name,
    legal_name: c.legal_name,
    party_type: c.party_type,
    is_foreign: c.is_foreign,
    country: c.country,
    tax_id_kind: c.tax_id_kind,
    tax_id: c.tax_id,
    is_pkp: c.is_pkp,
    vat_collector: c.vat_collector,
    email: c.email,
    phone: c.phone,
    payment_days: c.payment_days,
    credit_limit: c.credit_limit?.replace(/\.00$/, "") ?? null,
    notes: c.notes,
  };
}

/** Which tax IDs fit a party: an NPWP for a company, a NIK or NPWP for a person, a passport abroad. */
function kindsFor(party: PartyType, foreign: boolean): TaxIDKind[] {
  if (foreign) return party === "individual" ? ["passport", "none"] : ["none", "npwp"];
  if (party === "individual") return ["nik", "npwp", "none"];
  return ["npwp", "none"];
}

function duplicatesOf(err: unknown): Duplicate[] | null {
  if (!axios.isAxiosError(err) || err.response?.status !== 409) return null;
  const errors = (err.response.data as { errors?: unknown })?.errors;
  return Array.isArray(errors) && errors.every((e) => e && typeof (e as Duplicate).code === "string") ? (errors as Duplicate[]) : null;
}

export function CustomerFormSheet({ open, onClose, customer, canCredit = true, onSaved }: CustomerFormSheetProps) {
  const [values, setValues] = useState<CustomerValues>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [duplicates, setDuplicates] = useState<Duplicate[] | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setValues(customer ? valuesOf(customer) : EMPTY);
    setErrors({});
    setFormError(null);
    setDuplicates(null);
  }, [open, customer]);

  const set = <K extends keyof CustomerValues>(k: K) => (v: CustomerValues[K]) => {
    setValues((s) => ({ ...s, [k]: v }));
    if (k === "tax_id" || k === "tax_id_kind") setDuplicates(null);
  };

  // A party change narrows the tax ID kinds; keep the kind if it still fits.
  function setParty(party: PartyType, foreign: boolean) {
    setValues((s) => {
      const kinds = kindsFor(party, foreign);
      const kind = kinds.includes(s.tax_id_kind) ? s.tax_id_kind : kinds[0];
      return {
        ...s,
        party_type: party,
        is_foreign: foreign,
        country: foreign ? (s.country === "ID" ? "" : s.country) : "ID",
        tax_id_kind: kind,
        tax_id: kind === s.tax_id_kind ? s.tax_id : "",
        is_pkp: kind === "npwp" || kind === "nik" ? s.is_pkp : false,
        vat_collector: party === "government" || s.vat_collector !== "government" ? s.vat_collector : "none",
      };
    });
    setDuplicates(null);
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setErrors({});
    setFormError(null);
    try {
      const body = { ...values, confirm_duplicate: duplicates !== null };
      const saved = customer ? await updateCustomer(customer.id, body) : await createCustomer(body);
      notify.success(customer ? "Pelanggan disimpan" : `Pelanggan ${saved.customer.code} ditambahkan`);
      saved.warnings.forEach((w) => notify.warning(w));
      onSaved(saved.customer);
      onClose();
    } catch (err) {
      const dups = duplicatesOf(err);
      if (dups) {
        setDuplicates(dups);
        return;
      }
      const fields = apiFieldErrors(err);
      if (Object.keys(fields).length) setErrors(fields);
      else setFormError(apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  const kinds = kindsFor(values.party_type, values.is_foreign);
  const digits = values.tax_id_kind === "npwp" || values.tax_id_kind === "nik";
  const masked = customer?.tax_id_masked && values.tax_id === customer.tax_id && values.tax_id_kind === customer.tax_id_kind;

  return (
    <FormSheet
      open={open}
      onClose={() => !saving && onClose()}
      size="lg"
      title={customer ? `Ubah ${customer.name}` : "Pelanggan baru"}
      description={customer ? undefined : "Kode dibuat otomatis bila dikosongkan."}
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button type="submit" form="customer-form" loading={saving}>
            {duplicates ? "Tetap simpan" : customer ? "Simpan" : "Tambah pelanggan"}
          </Button>
        </>
      }
    >
      <form id="customer-form" onSubmit={submit} noValidate className="grid gap-6">
        {formError && <Notice tone="danger" title="Gagal menyimpan">{formError}</Notice>}
        {duplicates && (
          <Notice tone="warning" title="Identitas pajak ini sudah dipakai">
            <p>Satu NPWP boleh punya beberapa tempat usaha yang ditagih terpisah. Pastikan ini bukan pelanggan yang sama:</p>
            <ul className="mt-2 list-disc pl-5">
              {duplicates.map((d) => (
                <li key={d.id}>
                  <span className="font-mono">{d.code}</span> {d.name}
                  {!d.is_active && " (nonaktif)"}
                </li>
              ))}
            </ul>
          </Notice>
        )}

        <fieldset className="grid gap-4">
          <legend className="mb-1 text-sm font-semibold text-foreground">Identitas</legend>
          <FieldGrid columns={2}>
            <FormField label="Kode" required={!!customer} error={errors.code} hint={customer ? undefined : "Kosongkan untuk kode otomatis."}>
              <Input value={values.code} onChange={(e) => set("code")(e.target.value.toUpperCase())} maxLength={30} placeholder={customer ? undefined : "Otomatis"} />
            </FormField>
            <FormField label="Jenis" required error={errors.party_type}>
              <NativeSelect value={values.party_type} onChange={(e) => setParty(e.target.value as PartyType, values.is_foreign)}>
                {(Object.keys(PARTY_LABEL) as PartyType[]).map((p) => (
                  <NativeSelectOption key={p} value={p}>
                    {PARTY_LABEL[p]}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </FormField>
            <FormField label="Nama" required error={errors.name} hint="Tampil di daftar dan pemilih pelanggan.">
              <Input value={values.name} onChange={(e) => set("name")(e.target.value)} maxLength={150} placeholder="mis. Toko Maju Jaya" />
            </FormField>
            <FormField label="Nama resmi" error={errors.legal_name} hint="Seperti di faktur pajak; kosong memakai nama.">
              <Input value={values.legal_name} onChange={(e) => set("legal_name")(e.target.value)} maxLength={200} placeholder="mis. PT Maju Jaya Sentosa" />
            </FormField>
          </FieldGrid>
          <label className="flex items-center gap-2 text-sm">
            <Checkbox checked={values.is_foreign} onCheckedChange={(v) => setParty(values.party_type, v === true)} />
            Pelanggan luar negeri
          </label>
          {values.is_foreign && (
            <FormField label="Negara" required error={errors.country}>
              <SearchSelect options={COUNTRIES.filter((c) => c.value !== "ID")} value={values.country} onChange={(v) => set("country")(v == null ? "" : String(v))} />
            </FormField>
          )}
        </fieldset>

        <fieldset className="grid gap-4">
          <legend className="mb-1 text-sm font-semibold text-foreground">Pajak</legend>
          <FieldGrid columns={2}>
            <FormField label="Identitas pajak" error={errors.tax_id_kind}>
              <NativeSelect
                value={values.tax_id_kind}
                onChange={(e) => {
                  const kind = e.target.value as TaxIDKind;
                  setValues((s) => ({ ...s, tax_id_kind: kind, tax_id: "", is_pkp: kind === "npwp" || kind === "nik" ? s.is_pkp : false }));
                  setDuplicates(null);
                }}
              >
                {kinds.map((k) => (
                  <NativeSelectOption key={k} value={k}>
                    {TAX_ID_LABEL[k]}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </FormField>
            {values.tax_id_kind !== "none" && (
              <FormField
                label={TAX_ID_LABEL[values.tax_id_kind]}
                required
                error={errors.tax_id}
                hint={
                  masked
                    ? "Tersamar — biarkan untuk tetap memakai nomor yang tersimpan."
                    : values.tax_id_kind === "npwp"
                      ? "16 digit; NPWP lama 15 digit diubah otomatis."
                      : values.tax_id_kind === "nik"
                        ? "16 digit NIK — berlaku sebagai NPWP perorangan."
                        : undefined
                }
              >
                {digits && !masked ? (
                  <DigitsInput value={values.tax_id} onChange={set("tax_id")} maxDigits={16} groups={[4, 4, 4, 4]} />
                ) : (
                  <Input value={values.tax_id} onChange={(e) => set("tax_id")(e.target.value.toUpperCase())} maxLength={20} />
                )}
              </FormField>
            )}
          </FieldGrid>
          <label className="flex items-center gap-2 text-sm">
            <Checkbox checked={values.is_pkp} disabled={!digits} onCheckedChange={(v) => set("is_pkp")(v === true)} />
            Pengusaha Kena Pajak (PKP)
          </label>
          {errors.is_pkp && <p className="text-xs font-medium text-destructive">{errors.is_pkp}</p>}
          {!values.is_foreign && (
            <FormField label="Pemungut PPN" error={errors.vat_collector}>
              <NativeSelect value={values.vat_collector} onChange={(e) => set("vat_collector")(e.target.value as VATCollector)}>
                {(Object.keys(COLLECTOR_LABEL) as VATCollector[])
                  .filter((k) => k !== "government" || values.party_type === "government")
                  .map((k) => (
                    <NativeSelectOption key={k} value={k}>
                      {COLLECTOR_LABEL[k]}
                    </NativeSelectOption>
                  ))}
              </NativeSelect>
            </FormField>
          )}
        </fieldset>

        <fieldset className="grid gap-4">
          <legend className="mb-1 text-sm font-semibold text-foreground">Kontak</legend>
          <FieldGrid columns={2}>
            <FormField label="Email" error={errors.email}>
              <Input type="email" value={values.email} onChange={(e) => set("email")(e.target.value)} maxLength={190} />
            </FormField>
            <FormField label="Telepon" error={errors.phone}>
              <Input type="tel" value={values.phone} onChange={(e) => set("phone")(e.target.value)} maxLength={30} />
            </FormField>
          </FieldGrid>
        </fieldset>

        <fieldset className="grid gap-4">
          <legend className="mb-1 text-sm font-semibold text-foreground">Dagang</legend>
          {!canCredit && <Notice tone="info" title="Hanya bisa dilihat">Termin dan batas kredit diubah oleh pemegang izin customers.credit di seluruh perusahaan.</Notice>}
          <FieldGrid columns={2}>
            <FormField label="Termin pembayaran (hari)" error={errors.payment_days} hint="Kosong: tunai.">
              <DigitsInput
                value={values.payment_days == null ? "" : String(values.payment_days)}
                onChange={(d) => set("payment_days")(d === "" ? null : Number(d))}
                maxDigits={3}
                disabled={!canCredit}
              />
            </FormField>
            <FormField label="Batas kredit (Rp)" error={errors.credit_limit} hint="Kosong: tanpa batas. Ditegakkan oleh modul penjualan.">
              <Input
                inputMode="decimal"
                value={values.credit_limit ?? ""}
                onChange={(e) => {
                  const v = e.target.value.replace(/[^0-9.]/g, "");
                  set("credit_limit")(v === "" ? null : v);
                }}
                maxLength={20}
                disabled={!canCredit}
                className="tabular-nums"
              />
            </FormField>
          </FieldGrid>
        </fieldset>

        <FormField label="Catatan" error={errors.notes}>
          <Textarea value={values.notes} onChange={(e) => set("notes")(e.target.value)} rows={3} maxLength={2000} />
        </FormField>
      </form>
    </FormSheet>
  );
}
