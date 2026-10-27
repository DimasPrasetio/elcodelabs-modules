import { httpClient } from "@/shared/services/http-client";
import type { ApiPage, ApiSuccess } from "@/shared/types/api";
import type {
  Address,
  AddressValues,
  Contact,
  ContactValues,
  Customer,
  CustomerDetail,
  CustomerSettings,
  CustomerStatus,
  CustomerValues,
  Saved,
} from "@/modules/customers/types/customer";

const BASE = "/api/v1/customers";

const data = <T,>(p: Promise<{ data: ApiSuccess<T> }>) => p.then((r) => r.data.data);

export async function listCustomers(params: { page: number; limit: number; q: string; status: CustomerStatus }, signal?: AbortSignal): Promise<ApiPage<Customer>> {
  const { data } = await httpClient.get<ApiPage<Customer>>(BASE, { params, signal });
  return data;
}

export const fetchCustomer = (id: number, signal?: AbortSignal) => data<CustomerDetail>(httpClient.get(`${BASE}/${id}`, { signal }));
export const createCustomer = (values: CustomerValues) => data<Saved>(httpClient.post(BASE, values));
export const updateCustomer = (id: number, values: CustomerValues) => data<Saved>(httpClient.put(`${BASE}/${id}`, values));
export const setCustomerActive = (id: number, active: boolean) =>
  httpClient.post(`${BASE}/${id}/${active ? "activate" : "deactivate"}`).then(() => undefined);
export const holdCustomer = (id: number, reason: string) => httpClient.post(`${BASE}/${id}/hold`, { reason }).then(() => undefined);
export const releaseCustomer = (id: number) => httpClient.post(`${BASE}/${id}/release`).then(() => undefined);
/** The whole NIK or passport number — recorded as an access to personal data. */
export const revealTaxID = (id: number) => data<{ tax_id_kind: string; tax_id: string }>(httpClient.get(`${BASE}/${id}/reveal`));

export const saveAddress = (customerId: number, values: AddressValues, id?: number) =>
  data<Address>(id ? httpClient.put(`${BASE}/${customerId}/addresses/${id}`, values) : httpClient.post(`${BASE}/${customerId}/addresses`, values));
export const setAddressActive = (customerId: number, id: number, active: boolean) =>
  httpClient.post(`${BASE}/${customerId}/addresses/${id}/${active ? "activate" : "deactivate"}`).then(() => undefined);

export const saveContact = (customerId: number, values: ContactValues, id?: number) =>
  data<Contact>(id ? httpClient.put(`${BASE}/${customerId}/contacts/${id}`, values) : httpClient.post(`${BASE}/${customerId}/contacts`, values));
export const deleteContact = (customerId: number, id: number) => httpClient.delete(`${BASE}/${customerId}/contacts/${id}`).then(() => undefined);

export const fetchSettings = (signal?: AbortSignal) => data<CustomerSettings>(httpClient.get(`${BASE}/settings`, { signal }));
export const saveSettings = (codePrefix: string) => httpClient.put(`${BASE}/settings`, { code_prefix: codePrefix }).then(() => undefined);
