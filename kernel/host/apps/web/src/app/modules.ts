// Every frontend module — the one place a module is registered.
// Tooling inserts lines between the elco:* markers. Keep the markers; add modules between
// them, never around them.
import type { FrontendModule } from "@/shared/types/frontend-module";
// elco:modules:imports:begin
import { authModule } from "@/modules/auth/module";
import { dashboardModule } from "@/modules/dashboard/module";
// elco:modules:imports:end

export const frontendModules: FrontendModule[] = [
  // elco:modules:begin
  authModule,
  dashboardModule,
  // elco:modules:end
];
