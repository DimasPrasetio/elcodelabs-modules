import { httpClient } from "@/shared/services/http-client";
import type { ApiSuccess } from "@/shared/types/api";
import type { Branch, BranchValues, Company, CompanyValues, Me, OrgUser, Person } from "@/modules/organization/types/organization";

const BASE = "/api/v1/organization";

const data = <T,>(p: Promise<{ data: ApiSuccess<T> }>) => p.then((r) => r.data.data);

export const fetchMe = () => data<Me>(httpClient.get(`${BASE}/me`));
export const setMyDefault = (companyId: number, branchId: number) =>
  httpClient.put(`${BASE}/me/default`, { company_id: companyId, branch_id: branchId }).then(() => undefined);

export const fetchCompanies = (signal?: AbortSignal) => data<Company[]>(httpClient.get(`${BASE}/companies`, { signal }));
export const createCompany = (values: CompanyValues) => data<Company>(httpClient.post(`${BASE}/companies`, values));
export const updateCompany = (id: number, values: CompanyValues) => data<Company>(httpClient.put(`${BASE}/companies/${id}`, values));
export const setCompanyActive = (id: number, active: boolean) =>
  httpClient.post(`${BASE}/companies/${id}/${active ? "activate" : "deactivate"}`).then(() => undefined);
export const fetchUnplaced = (signal?: AbortSignal) => data<OrgUser[]>(httpClient.get(`${BASE}/unplaced`, { signal }));

export const fetchBranches = (signal?: AbortSignal) => data<Branch[]>(httpClient.get(`${BASE}/branches`, { signal }));
export const createBranch = (values: BranchValues) => data<Branch>(httpClient.post(`${BASE}/branches`, values));
export const updateBranch = (id: number, values: BranchValues) => data<Branch>(httpClient.put(`${BASE}/branches/${id}`, values));
export const setBranchActive = (id: number, active: boolean) =>
  httpClient.post(`${BASE}/branches/${id}/${active ? "activate" : "deactivate"}`).then(() => undefined);

export const fetchPeople = (signal?: AbortSignal) => data<Person[]>(httpClient.get(`${BASE}/placements`, { signal }));
/** Places a user in the active company — across it (branchId 0) or in one branch. */
export const placeUser = (userId: number, branchId: number) =>
  httpClient.post(`${BASE}/placements`, { user_id: userId, branch_id: branchId }).then(() => undefined);
export const unplaceUser = (userId: number, branchId: number) =>
  httpClient.delete(`${BASE}/placements/${userId}`, { params: { branch: branchId } }).then(() => undefined);
export const searchCandidates = (q: string) => data<OrgUser[]>(httpClient.get(`${BASE}/candidates`, { params: { q } }));
/** From the home company: places someone, by exact email, across another company. */
export const placeInCompany = (companyId: number, email: string) =>
  httpClient.post(`${BASE}/companies/${companyId}/placements`, { email }).then(() => undefined);
