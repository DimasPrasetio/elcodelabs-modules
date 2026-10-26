import { create } from "zustand";
import { fetchHealth } from "@/modules/dashboard/services/health.service";
import type { HealthResponse } from "@/modules/dashboard/services/health.service";

interface HealthState {
  status: "idle" | "loading" | "success" | "error";
  result: HealthResponse | null;
  error: string | null;
  check: () => Promise<void>;
}

export const useHealthStore = create<HealthState>((set) => ({
  status: "idle",
  result: null,
  error: null,
  check: async () => {
    set({ status: "loading", error: null });
    try {
      const result = await fetchHealth();
      set({ status: "success", result });
    } catch (err) {
      set({ status: "error", error: err instanceof Error ? err.message : "Kesalahan tidak diketahui" });
    }
  },
}));
