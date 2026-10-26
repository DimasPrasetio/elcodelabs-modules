import { lazy } from "react";
import type { FrontendModule } from "@/shared/types/frontend-module";

const LoginPage = lazy(() => import("@/modules/auth/pages/LoginPage"));

export const AUTH_PATHS = {
  login: "/login",
} as const;

export const authModule: FrontendModule = {
  id: "auth",
  public: [{ path: AUTH_PATHS.login, element: <LoginPage /> }],
};
