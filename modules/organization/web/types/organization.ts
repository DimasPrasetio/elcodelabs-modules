/** One place the signed-in user may work. */
export interface Unit {
  company_id: number;
  company_code: string;
  company_name: string;
  /** 0: across the whole company. */
  branch_id: number;
  branch_code: string;
  branch_name: string;
  /** A placement of the user's own (it can be the default), not a branch of a company placed across. */
  placed: boolean;
}

export interface Me {
  units: Unit[];
  default: Unit;
  current: { company_id: number; branch_id: number; timezone: string };
}

export interface Company {
  id: number;
  code: string;
  name: string;
  timezone: string;
  is_active: boolean;
  created_at: string;
}

export interface CompanyValues {
  code: string;
  name: string;
  timezone: string;
}

export interface Branch {
  id: number;
  code: string;
  name: string;
  /** "" — the company's. */
  timezone: string;
  nitku: string;
  address: string;
  is_active: boolean;
  created_at: string;
}

export interface BranchValues {
  code: string;
  name: string;
  timezone: string;
  nitku: string;
  address: string;
}

export interface OrgUser {
  id: number;
  name: string;
  email: string;
  is_active: boolean;
}

export interface Person extends OrgUser {
  /** 0: across the company. */
  branch_id: number;
  is_default: boolean;
  /** The user directory no longer knows them. */
  missing: boolean;
}

/** The zones offered in forms; any IANA zone stored through the API is shown as it is. */
export const ZONES = [
  { value: "Asia/Jakarta", label: "WIB — Asia/Jakarta" },
  { value: "Asia/Makassar", label: "WITA — Asia/Makassar" },
  { value: "Asia/Jayapura", label: "WIT — Asia/Jayapura" },
];

/** "WITA · Asia/Makassar" for a zone offered here; the IANA name otherwise. */
export const zoneLabel = (zone: string) => {
  const known = ZONES.find((z) => z.value === zone);
  return known ? `${known.label.split(" — ")[0]} · ${zone}` : zone;
};

/** "<company>" or "<company>:<branch>" — what @/shared/scope sends as X-Scope. */
export const unitKey = (companyId: number, branchId: number) => (branchId ? `${companyId}:${branchId}` : `${companyId}`);
