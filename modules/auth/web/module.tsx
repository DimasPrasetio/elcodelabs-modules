import { lazy } from "react";
import type { FrontendModule } from "@/shared/types/frontend-module";
import { RequireAuth } from "@/modules/auth/components/RequireAuth";
import { UserMenu } from "@/modules/auth/components/UserMenu";
import { AUTH_PATHS } from "@/modules/auth/paths";

const LoginPage = lazy(() => import("@/modules/auth/pages/LoginPage"));

export { AUTH_PATHS };

export const authModule: FrontendModule = {
  id: "auth",
  public: [{ path: AUTH_PATHS.login, element: <LoginPage /> }],
  guard: <RequireAuth />,
  slots: { "sidebar-footer": <UserMenu /> },
};
