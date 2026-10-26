import { httpClient } from "@/shared/services/http-client";
import type { ApiSuccess } from "@/shared/types/api";
import type { Permission, RbacMember, RbacUser, Role } from "@/modules/rbac/types/rbac";

const BASE = "/api/v1/rbac";

export interface RoleValues {
  name: string;
  description: string;
  permissions: string[];
}

const data = <T,>(p: Promise<{ data: ApiSuccess<T> }>) => p.then((r) => r.data.data);

export const fetchMyPermissions = () => data<{ permissions: string[] }>(httpClient.get(`${BASE}/me`)).then((d) => d.permissions);
export const fetchCatalog = () => data<Permission[]>(httpClient.get(`${BASE}/permissions`));
export const fetchRoles = (signal?: AbortSignal) => data<Role[]>(httpClient.get(`${BASE}/roles`, { signal }));
export const fetchRole = (id: number) => data<Role>(httpClient.get(`${BASE}/roles/${id}`));
export const createRole = (values: RoleValues) => data<Role>(httpClient.post(`${BASE}/roles`, values));
export const updateRole = (id: number, values: RoleValues) => data<Role>(httpClient.put(`${BASE}/roles/${id}`, values));
export const deleteRole = (id: number) => httpClient.delete(`${BASE}/roles/${id}`).then(() => undefined);
export const fetchMembers = (id: number) => data<RbacMember[]>(httpClient.get(`${BASE}/roles/${id}/members`));
/** Gives the role in the active company — everywhere (branchId 0) or in one branch. */
export const addMember = (id: number, userId: number, branchId = 0) =>
  httpClient.post(`${BASE}/roles/${id}/members`, { user_id: userId, branch_id: branchId }).then(() => undefined);
export const removeMember = (id: number, userId: number, branchId = 0) =>
  httpClient.delete(`${BASE}/roles/${id}/members/${userId}`, { params: { branch: branchId } }).then(() => undefined);
export const searchUsers = (q: string) => data<RbacUser[]>(httpClient.get(`${BASE}/users`, { params: { q } }));
