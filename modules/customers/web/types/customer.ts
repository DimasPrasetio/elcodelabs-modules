export type PartyType = "company" | "individual" | "government";
export type TaxIDKind = "npwp" | "nik" | "passport" | "none";
export type VATCollector = "none" | "government" | "other";
export type CustomerStatus = "" | "active" | "on_hold" | "inactive";

export interface Customer {
  id: number;
  code: string;
  name: string;
  legal_name: string;
  party_type: PartyType;
  is_foreign: boolean;
  country: string;
  tax_id_kind: TaxIDKind;
  /** NIK and passport numbers come masked (tax_id_masked); the whole one through reveal. */
  tax_id: string;
  tax_id_masked: boolean;
  is_pkp: boolean;
  vat_collector: VATCollector;
  email: string;
  phone: string;
  payment_days: number | null;
  /** Decimal string, e.g. "50000000.00"; null: no limit. */
  credit_limit: string | null;
  on_hold: boolean;
  hold_reason: string;
  notes: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CustomerValues {
  code: string;
  name: string;
  legal_name: string;
  party_type: PartyType;
  is_foreign: boolean;
  country: string;
  tax_id_kind: TaxIDKind;
  tax_id: string;
  is_pkp: boolean;
  vat_collector: VATCollector;
  email: string;
  phone: string;
  payment_days: number | null;
  credit_limit: string | null;
  notes: string;
  confirm_duplicate?: boolean;
}

export interface Address {
  id: number;
  label: string;
  line: string;
  city: string;
  province: string;
  postal_code: string;
  country: string;
  nitku: string;
  for_billing: boolean;
  for_shipping: boolean;
  is_default_billing: boolean;
  is_default_shipping: boolean;
  is_active: boolean;
}

export type AddressValues = Omit<Address, "id" | "is_active">;

export interface Contact {
  id: number;
  name: string;
  title: string;
  email: string;
  phone: string;
  is_primary: boolean;
}

export type ContactValues = Omit<Contact, "id">;

export interface CustomerDetail {
  customer: Customer;
  addresses: Address[];
  contacts: Contact[];
  /** What needs a permission across the whole company. */
  can: { credit: boolean; sensitive: boolean; settings: boolean };
}

export interface Saved {
  customer: Customer;
  warnings: string[];
}

export interface Duplicate {
  id: number;
  code: string;
  name: string;
  is_active: boolean;
}

export interface CustomerSettings {
  code_prefix: string;
  next_code: string;
  can_manage: boolean;
}

export const PARTY_LABEL: Record<PartyType, string> = {
  company: "Badan usaha",
  individual: "Perorangan",
  government: "Instansi pemerintah",
};

export const TAX_ID_LABEL: Record<TaxIDKind, string> = {
  npwp: "NPWP",
  nik: "NIK",
  passport: "Paspor",
  none: "Tanpa identitas pajak",
};

export const COLLECTOR_LABEL: Record<VATCollector, string> = {
  none: "Bukan pemungut PPN",
  government: "Pemungut PPN — instansi pemerintah",
  other: "Pemungut PPN lainnya (mis. BUMN)",
};

export const COUNTRIES = [
  { value: "ID", label: "Indonesia" },
  { value: "SG", label: "Singapura" },
  { value: "MY", label: "Malaysia" },
  { value: "TH", label: "Thailand" },
  { value: "VN", label: "Vietnam" },
  { value: "PH", label: "Filipina" },
  { value: "CN", label: "Tiongkok" },
  { value: "JP", label: "Jepang" },
  { value: "KR", label: "Korea Selatan" },
  { value: "AU", label: "Australia" },
  { value: "US", label: "Amerika Serikat" },
  { value: "GB", label: "Inggris" },
  { value: "DE", label: "Jerman" },
  { value: "NL", label: "Belanda" },
  { value: "SA", label: "Arab Saudi" },
  { value: "AE", label: "Uni Emirat Arab" },
];

export const countryLabel = (code: string) => COUNTRIES.find((c) => c.value === code)?.label ?? code;

/** "30 hari" or "Tunai". */
export const termsLabel = (days: number | null) => (days == null ? "Tunai" : days === 0 ? "Tunai (0 hari)" : `${days} hari`);
