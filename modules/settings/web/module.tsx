import { lazy } from "react";
import { Settings } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";

const SettingsPage = lazy(() => import("@/modules/settings/pages/SettingsPage"));

export const SETTINGS_PATHS = {
  root: "/settings",
} as const;

export const settingsModule: FrontendModule = {
  id: "settings",
  protected: [{ path: SETTINGS_PATHS.root, element: <SettingsPage /> }],
  nav: [{ label: "Umum", to: SETTINGS_PATHS.root, icon: Settings, group: "Pengaturan", order: 800, permission: "settings.manage" }],
};
