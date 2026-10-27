import { httpClient } from "@/shared/services/http-client";
import type { ApiPage, ApiSuccess } from "@/shared/types/api";
import type {
  Address,
  BankAccount,
  BankAccountValues,
  AddressValues,
  Contact,
  ContactValues,
  Supplier,
  SupplierDetail,
  SupplierSettings,
  SupplierStatus,
  SupplierValues,
  Revealed,
  Saved,
} from "@/modules/suppliers/types/supplier";

const BASE = "/api/v1/suppliers";

const data = <T,>(p: Promise<{ data: ApiSuccess<T> }>) => p.then((r) => r.data.data);

export async function listSuppliers(params: { page: number; limit: number; q: string; status: SupplierStatus }, signal?: AbortSignal): Promise<ApiPage<Supplier>> {
  const { data } = await httpClient.get<ApiPage<Supplier>>(BASE, { params, signal });
  return data;
}

export const fetchSupplier = (id: number, signal?: AbortSignal) => data<SupplierDetail>(httpClient.get(`${BASE}/${id}`, { signal }));
export const createSupplier = (values: SupplierValues) => data<Saved>(httpClient.post(BASE, values));
export const updateSupplier = (id: number, values: SupplierValues) => data<Saved>(httpClient.put(`${BASE}/${id}`, values));
export const setSupplierActive = (id: number, active: boolean) =>
  httpClient.post(`${BASE}/${id}/${active ? "activate" : "deactivate"}`).then(() => undefined);
export const holdSupplier = (id: number, reason: string) => httpClient.post(`${BASE}/${id}/hold`, { reason }).then(() => undefined);
export const releaseSupplier = (id: number) => httpClient.post(`${BASE}/${id}/release`).then(() => undefined);
/** The whole tax ID and bank account numbers — recorded as an access to personal data. */
export const revealSensitive = (id: number) => data<Revealed>(httpClient.get(`${BASE}/${id}/reveal`));

export const saveBankAccount = (supplierId: number, values: BankAccountValues, id?: number) =>
  data<{ bank_account: BankAccount; warnings: string[] }>(
    id ? httpClient.put(`${BASE}/${supplierId}/bank-accounts/${id}`, values) : httpClient.post(`${BASE}/${supplierId}/bank-accounts`, values),
  );
export const setBankAccountActive = (supplierId: number, id: number, active: boolean) =>
  httpClient.post(`${BASE}/${supplierId}/bank-accounts/${id}/${active ? "activate" : "deactivate"}`).then(() => undefined);

export const saveAddress = (supplierId: number, values: AddressValues, id?: number) =>
  data<Address>(id ? httpClient.put(`${BASE}/${supplierId}/addresses/${id}`, values) : httpClient.post(`${BASE}/${supplierId}/addresses`, values));
export const setAddressActive = (supplierId: number, id: number, active: boolean) =>
  httpClient.post(`${BASE}/${supplierId}/addresses/${id}/${active ? "activate" : "deactivate"}`).then(() => undefined);

export const saveContact = (supplierId: number, values: ContactValues, id?: number) =>
  data<Contact>(id ? httpClient.put(`${BASE}/${supplierId}/contacts/${id}`, values) : httpClient.post(`${BASE}/${supplierId}/contacts`, values));
export const deleteContact = (supplierId: number, id: number) => httpClient.delete(`${BASE}/${supplierId}/contacts/${id}`).then(() => undefined);

export const fetchSettings = (signal?: AbortSignal) => data<SupplierSettings>(httpClient.get(`${BASE}/settings`, { signal }));
export const saveSettings = (codePrefix: string) => httpClient.put(`${BASE}/settings`, { code_prefix: codePrefix }).then(() => undefined);
