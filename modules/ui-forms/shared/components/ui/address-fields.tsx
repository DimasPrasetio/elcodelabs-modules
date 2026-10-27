import { Input } from "@/shared/components/primitives/input";
import { Textarea } from "@/shared/components/primitives/textarea";
import { FieldGrid } from "@/shared/components/ui/form-layout";
import { FormField } from "@/shared/components/ui/form-field";
import { SearchSelect } from "@/shared/components/ui/search-select";
import type { SelectOption } from "@/shared/components/ui/select-parts";

export interface AddressValue {
  line: string;
  city: string;
  province: string;
  postal_code: string;
  /** ISO 3166-1 alpha-2, e.g. "ID". */
  country: string;
}

export const EMPTY_ADDRESS: AddressValue = { line: "", city: "", province: "", postal_code: "", country: "ID" };

export interface AddressLabels {
  line: string;
  city: string;
  province: string;
  postal_code: string;
  country: string;
}

const DEFAULT_LABELS: AddressLabels = {
  line: "Alamat",
  city: "Kota/kabupaten",
  province: "Provinsi",
  postal_code: "Kode pos",
  country: "Negara",
};

export interface AddressFieldsProps {
  value: AddressValue;
  onChange: (value: AddressValue) => void;
  /** Field errors keyed by the value's keys (a 422 list read with apiFieldErrors). */
  errors?: Partial<Record<keyof AddressValue, string>>;
  /** Without it there is no country field and the value keeps its country. */
  countries?: SelectOption[];
  labels?: Partial<AddressLabels>;
  /** Fields that must be filled; "line" by default. */
  required?: (keyof AddressValue)[];
  disabled?: boolean;
}

/** A postal address as separate fields: street, city, province, postal code, and country. */
export function AddressFields({ value, onChange, errors = {}, countries, labels, required = ["line"], disabled }: AddressFieldsProps) {
  const l = { ...DEFAULT_LABELS, ...labels };
  const set = (k: keyof AddressValue) => (v: string) => onChange({ ...value, [k]: v });
  const req = (k: keyof AddressValue) => required.includes(k);
  return (
    <div className="grid gap-4">
      <FormField label={l.line} required={req("line")} error={errors.line}>
        <Textarea value={value.line} onChange={(e) => set("line")(e.target.value)} rows={2} maxLength={255} disabled={disabled} />
      </FormField>
      <FieldGrid columns={2}>
        <FormField label={l.city} required={req("city")} error={errors.city}>
          <Input value={value.city} onChange={(e) => set("city")(e.target.value)} maxLength={100} disabled={disabled} />
        </FormField>
        <FormField label={l.province} required={req("province")} error={errors.province}>
          <Input value={value.province} onChange={(e) => set("province")(e.target.value)} maxLength={100} disabled={disabled} />
        </FormField>
        <FormField label={l.postal_code} required={req("postal_code")} error={errors.postal_code}>
          <Input
            value={value.postal_code}
            onChange={(e) => set("postal_code")(e.target.value.replace(/[^0-9A-Za-z -]/g, ""))}
            maxLength={10}
            disabled={disabled}
          />
        </FormField>
        {countries && (
          <FormField label={l.country} required={req("country")} error={errors.country}>
            <SearchSelect options={countries} value={value.country} onChange={(v) => set("country")(v == null ? "" : String(v))} disabled={disabled} />
          </FormField>
        )}
      </FieldGrid>
    </div>
  );
}
