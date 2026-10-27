export interface Company {
  /** The company this profile belongs to — the request's scope. */
  company_id: number;
  name: string;
  legal_name: string;
  address: string;
  phone: string;
  email: string;
  tax_id: string;
  timezone: string;
  has_logo: boolean;
  logo_upload: boolean;
}

export type CompanyValues = Omit<Company, "company_id" | "has_logo" | "logo_upload">;
