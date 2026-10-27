import { httpClient } from "@/shared/services/http-client";
import type { ApiSuccess } from "@/shared/types/api";
import type { PeriodOverview } from "@/modules/period/types/period";

const BASE = "/api/v1/period";

export async function fetchOverview(signal?: AbortSignal): Promise<PeriodOverview> {
  const { data } = await httpClient.get<ApiSuccess<PeriodOverview>>(`${BASE}/locks`, { signal });
  return data.data;
}

/** Moves an area's lock; lockedThrough null removes it. Reopening needs a reason. */
export async function setLock(area: string, lockedThrough: string | null, reason: string): Promise<void> {
  await httpClient.put(`${BASE}/locks/${encodeURIComponent(area)}`, { locked_through: lockedThrough, reason });
}

export async function setFiscalYearStart(month: number): Promise<void> {
  await httpClient.put(`${BASE}/fiscal-year`, { fiscal_year_start_month: month });
}
