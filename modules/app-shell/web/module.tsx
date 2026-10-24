import type { FrontendModule } from "@/shared/types/frontend-module";
import { AppShell } from "@/modules/app-shell/components/AppShell";

export const appShellModule: FrontendModule = {
  id: "app-shell",
  layout: <AppShell />,
};
