import { httpClient } from "@/shared/services/http-client";
import type { ApiPage } from "@/shared/types/api";
import type { OutboxMessage } from "@/modules/mailer/types/mail";

const BASE = "/api/v1/mailer";

export async function listMessages(params: { page: number; limit: number; q: string; status: string }, signal?: AbortSignal): Promise<ApiPage<OutboxMessage>> {
  const { data } = await httpClient.get<ApiPage<OutboxMessage>>(`${BASE}/messages`, { params, signal });
  return data;
}

export async function retryMessage(id: number): Promise<void> {
  await httpClient.post(`${BASE}/messages/${id}/retry`, {});
}
