import { lazy } from "react";
import { Users } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";

const UsersPage = lazy(() => import("@/modules/users/pages/UsersPage"));

export const USERS_PATHS = {
  list: "/users",
} as const;

export const usersModule: FrontendModule = {
  id: "users",
  protected: [{ path: USERS_PATHS.list, element: <UsersPage /> }],
  nav: [{ label: "Users", to: USERS_PATHS.list, icon: Users, group: "Settings", order: 900, permission: "users.read" }],
};
