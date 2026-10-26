import { lazy } from "react";
import type { FrontendModule } from "@/shared/types/frontend-module";
import { RequireAuth } from "@/modules/auth/components/RequireAuth";
import { UserMenu } from "@/modules/auth/components/UserMenu";
import { AUTH_PATHS } from "@/modules/auth/paths";

const LoginPage = lazy(() => import("@/modules/auth/pages/LoginPage"));
const ForgotPasswordPage = lazy(() => import("@/modules/auth/pages/ForgotPasswordPage"));
const ResetPasswordPage = lazy(() => import("@/modules/auth/pages/ResetPasswordPage"));
const ChangePasswordPage = lazy(() => import("@/modules/auth/pages/ChangePasswordPage"));

export { AUTH_PATHS };

export const authModule: FrontendModule = {
  id: "auth",
  public: [
    { path: AUTH_PATHS.login, element: <LoginPage /> },
    { path: AUTH_PATHS.forgot, element: <ForgotPasswordPage /> },
    { path: AUTH_PATHS.reset, element: <ResetPasswordPage /> },
  ],
  protected: [{ path: AUTH_PATHS.password, element: <ChangePasswordPage /> }],
  guard: <RequireAuth />,
  slots: { "sidebar-footer": <UserMenu /> },
};
