import { httpClient } from "@/shared/services/http-client";
import type { ApiPage, ApiSuccess } from "@/shared/types/api";
import type { User } from "@/modules/users/types/user";
import type { CreateUserValues, UpdateUserValues } from "@/modules/users/schemas/user.schema";

const BASE = "/api/v1/users";

export async function listUsers(params: { page: number; limit: number; q: string }): Promise<ApiPage<User>> {
  const { data } = await httpClient.get<ApiPage<User>>(BASE, { params });
  return data;
}

export async function createUser(values: CreateUserValues): Promise<User> {
  const { data } = await httpClient.post<ApiSuccess<User>>(BASE, values);
  return data.data;
}

export async function updateUser(id: number, values: UpdateUserValues): Promise<User> {
  const { data } = await httpClient.put<ApiSuccess<User>>(`${BASE}/${id}`, values);
  return data.data;
}

export async function deleteUser(id: number): Promise<void> {
  await httpClient.delete(`${BASE}/${id}`);
}
