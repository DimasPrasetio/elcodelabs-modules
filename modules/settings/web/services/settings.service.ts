import { httpClient } from "@/shared/services/http-client";
import type { ApiSuccess } from "@/shared/types/api";
import type { Company, CompanyValues } from "@/modules/settings/types/settings";

const BASE = "/api/v1/settings/company";

/** A company's logo; ?v= changes after an upload so the browser does not show the old one. */
export const logoUrl = (companyId: number, version: number) => `${BASE}/logo?company=${companyId}&v=${version}`;

export async function fetchCompany(signal?: AbortSignal): Promise<Company> {
  const { data } = await httpClient.get<ApiSuccess<Company>>(BASE, { signal });
  return data.data;
}

export async function saveCompany(values: CompanyValues): Promise<Company> {
  const { data } = await httpClient.put<ApiSuccess<Company>>(BASE, values);
  return data.data;
}

export async function uploadLogo(file: File): Promise<Company> {
  const form = new FormData();
  form.append("file", file);
  const { data } = await httpClient.put<ApiSuccess<Company>>(`${BASE}/logo`, form, { headers: { "Content-Type": "multipart/form-data" } });
  return data.data;
}

export async function removeLogo(): Promise<Company> {
  const { data } = await httpClient.delete<ApiSuccess<Company>>(`${BASE}/logo`);
  return data.data;
}
