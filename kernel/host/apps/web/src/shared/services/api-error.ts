import axios from "axios";
import type { ApiFailure, ApiFieldError } from "@/shared/types/api";

function failure(err: unknown): ApiFailure | undefined {
  if (!axios.isAxiosError(err)) return undefined;
  const body = err.response?.data as Partial<ApiFailure> | undefined;
  return body && body.success === false ? (body as ApiFailure) : undefined;
}

/** The API's own message for a failed request, or a plain fallback when there is none. */
export function apiErrorMessage(err: unknown, fallback = "Permintaan gagal. Coba lagi."): string {
  return failure(err)?.message ?? (err instanceof Error && !axios.isAxiosError(err) ? err.message : fallback);
}

/** Field errors from a 422 (validator.FieldError on the backend), keyed by field name. */
export function apiFieldErrors(err: unknown): Record<string, string> {
  const out: Record<string, string> = {};
  for (const e of (failure(err)?.errors ?? []) as ApiFieldError[]) {
    if (e && typeof e.field === "string") out[e.field] ??= e.message;
  }
  return out;
}
