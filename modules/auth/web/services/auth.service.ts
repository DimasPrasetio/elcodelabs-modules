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

export interface AuthOptions {
  password_change: boolean;
  password_reset: boolean;
}

export async function fetchOptions(): Promise<AuthOptions> {
  const { data } = await httpClient.get<ApiSuccess<AuthOptions>>("/api/v1/auth/options");
  return data.data;
}

/** Changes the signed-in user's password; the API answers with a fresh session for this device. */
export async function changePassword(current_password: string, new_password: string): Promise<string> {
  const { data } = await httpClient.post<ApiSuccess<unknown>>("/api/v1/auth/password", { current_password, new_password });
  return data.message;
}

export async function requestReset(email: string): Promise<string> {
  const { data } = await httpClient.post<ApiSuccess<null>>("/api/v1/auth/forgot", { email });
  return data.message;
}

export async function resetPassword(token: string, new_password: string): Promise<string> {
  const { data } = await httpClient.post<ApiSuccess<null>>("/api/v1/auth/reset", { token, new_password });
  return data.message;
}
