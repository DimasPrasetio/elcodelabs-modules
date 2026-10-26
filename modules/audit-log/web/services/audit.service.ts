import { httpClient } from "@/shared/services/http-client";
import type { ApiPage, ApiSuccess } from "@/shared/types/api";
import type { AuditEntry } from "@/modules/audit-log/types/audit";

export interface AuditQuery {
  page: number;
  limit: number;
  kind: string;
  module: string;
  target: string;
  from: string;
  to: string;
}

export async function listAudit(q: AuditQuery, signal?: AbortSignal): Promise<ApiPage<AuditEntry>> {
  const params = Object.fromEntries(Object.entries(q).filter(([, v]) => v !== ""));
  const { data } = await httpClient.get<ApiPage<AuditEntry>>("/api/v1/audit", { params, signal });
  return data;
}

export async function listModules(signal?: AbortSignal): Promise<string[]> {
  const { data } = await httpClient.get<ApiSuccess<string[]>>("/api/v1/audit/modules", { signal });
  return data.data;
}
