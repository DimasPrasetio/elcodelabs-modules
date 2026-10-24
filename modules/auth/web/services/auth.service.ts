import { httpClient } from "@/shared/services/http-client";
import type { ApiSuccess } from "@/shared/types/api";
import type { LoginFormValues } from "@/modules/auth/schemas/login.schema";

export interface SessionUser {
  id: number;
  name: string;
  email: string;
}

// The session lives in an HttpOnly cookie the API sets; the browser sends it on its own, so
// no token is ever handled here.

export async function login(values: LoginFormValues): Promise<SessionUser> {
  const { data } = await httpClient.post<ApiSuccess<{ user: SessionUser }>>("/api/v1/auth/login", values);
  return data.data.user;
}

export async function logout(): Promise<void> {
  await httpClient.post("/api/v1/auth/logout");
}

export async function fetchMe(): Promise<SessionUser> {
  const { data } = await httpClient.get<ApiSuccess<SessionUser>>("/api/v1/auth/me");
  return data.data;
}
