import { httpClient } from "@/shared/services/http-client";
import type { ApiPage } from "@/shared/types/api";
import type { StoredFile } from "@/modules/files/types/file";

const BASE = "/api/v1/files";

export const downloadUrl = (id: number) => `${BASE}/${id}/download`;

export async function listFiles(params: { page: number; limit: number; q: string }, signal?: AbortSignal): Promise<ApiPage<StoredFile>> {
  const { data } = await httpClient.get<ApiPage<StoredFile>>(BASE, { params, signal });
  return data;
}

export async function deleteFile(id: number): Promise<void> {
  await httpClient.delete(`${BASE}/${id}`);
}
