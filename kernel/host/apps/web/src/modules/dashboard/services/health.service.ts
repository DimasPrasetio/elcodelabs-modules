import { httpClient } from "@/shared/services/http-client";

export interface HealthResponse {
  success: boolean;
  message: string;
  data: unknown;
}

export async function fetchHealth(): Promise<HealthResponse> {
  const { data } = await httpClient.get<HealthResponse>("/api/v1/health");
  return data;
}
