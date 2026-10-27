import { httpClient } from "@/shared/services/http-client";
import type { ApiSuccess } from "@/shared/types/api";
import type { NumberFormat, NumberingOverview } from "@/modules/numbering/types/numbering";

const BASE = "/api/v1/numbering";
const type = (docType: string) => `${BASE}/types/${encodeURIComponent(docType)}`;

export async function fetchOverview(signal?: AbortSignal): Promise<NumberingOverview> {
  const { data } = await httpClient.get<ApiSuccess<NumberingOverview>>(`${BASE}/types`, { signal });
  return data.data;
}

/** The format rendered for today at counter 1; a 422 says what is wrong with it. */
export async function sample(format: NumberFormat, signal?: AbortSignal): Promise<string> {
  const { data } = await httpClient.post<ApiSuccess<{ sample: string }>>(`${BASE}/sample`, format, { signal });
  return data.data.sample;
}

export async function saveFormat(docType: string, format: NumberFormat): Promise<void> {
  await httpClient.put(type(docType), format);
}

export async function resetFormat(docType: string): Promise<void> {
  await httpClient.delete(type(docType));
}

export async function raiseNext(docType: string, branchId: number, next: number, reason: string): Promise<void> {
  await httpClient.put(`${type(docType)}/next`, { branch_id: branchId, next, reason });
}
