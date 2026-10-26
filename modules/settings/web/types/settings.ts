export interface Company {
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

export type CompanyValues = Omit<Company, "has_logo" | "logo_upload">;
