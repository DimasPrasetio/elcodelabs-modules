export type PartyType = "company" | "individual" | "government";
export type TaxIDKind = "npwp" | "nik" | "passport" | "none";
export type SupplierStatus = "" | "active" | "on_hold" | "inactive";

export interface Supplier {
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
  email: string;
  phone: string;
  payment_days: number | null;
  on_hold: boolean;
  hold_reason: string;
  notes: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface SupplierValues {
  code: string;
  name: string;
  legal_name: string;
  party_type: PartyType;
  is_foreign: boolean;
  country: string;
  tax_id_kind: TaxIDKind;
  tax_id: string;
  is_pkp: boolean;
  email: string;
  phone: string;
  payment_days: number | null;
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

export interface BankAccount {
  id: number;
  bank_name: string;
  bank_code: string;
  /** Masked unless it came from reveal. */
  account_number: string;
  account_holder: string;
  is_default: boolean;
  is_active: boolean;
}

export type BankAccountValues = Omit<BankAccount, "id" | "is_active">;

export interface SupplierDetail {
  supplier: Supplier;
  addresses: Address[];
  bank_accounts: BankAccount[];
  contacts: Contact[];
  /** What needs a permission across the whole company. */
  can: { sensitive: boolean; settings: boolean };
}

export interface Revealed {
  tax_id_kind: string;
  tax_id: string;
  bank_accounts: BankAccount[];
}

/** Common Indonesian banks with their 3-digit codes — a suggestion list, not a rule. */
export const BANKS = [
  { code: "002", name: "BRI" },
  { code: "008", name: "Bank Mandiri" },
  { code: "009", name: "BNI" },
  { code: "011", name: "Bank Danamon" },
  { code: "013", name: "Bank Permata" },
  { code: "014", name: "BCA" },
  { code: "016", name: "Maybank Indonesia" },
  { code: "022", name: "CIMB Niaga" },
  { code: "028", name: "OCBC Indonesia" },
  { code: "200", name: "BTN" },
  { code: "426", name: "Bank Mega" },
  { code: "451", name: "Bank Syariah Indonesia" },
];

export interface Saved {
  supplier: Supplier;
  warnings: string[];
}

export interface Duplicate {
  id: number;
  code: string;
  name: string;
  is_active: boolean;
}

export interface SupplierSettings {
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

/** "30 hari" or "Tunai saat terima". */
export const termsLabel = (days: number | null) => (days == null ? "Tunai saat terima" : days === 0 ? "Tunai (0 hari)" : `${days} hari`);
