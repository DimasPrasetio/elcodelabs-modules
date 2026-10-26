import { httpClient } from "@/shared/services/http-client";
import type { ApiSuccess } from "@/shared/types/api";
import type { Permission, RbacUser, Role } from "@/modules/rbac/types/rbac";

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
export const fetchMembers = (id: number) => data<RbacUser[]>(httpClient.get(`${BASE}/roles/${id}/members`));
export const addMember = (id: number, userId: number) =>
  httpClient.post(`${BASE}/roles/${id}/members`, { user_id: userId }).then(() => undefined);
export const removeMember = (id: number, userId: number) =>
  httpClient.delete(`${BASE}/roles/${id}/members/${userId}`).then(() => undefined);
export const searchUsers = (q: string) => data<RbacUser[]>(httpClient.get(`${BASE}/users`, { params: { q } }));
