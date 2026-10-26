import { lazy } from "react";
import { Users } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";
import { ProfileMenuItem } from "@/modules/users/components/ProfileMenuItem";

const UsersPage = lazy(() => import("@/modules/users/pages/UsersPage"));
const AccountPage = lazy(() => import("@/modules/users/pages/AccountPage"));

export const USERS_PATHS = {
  list: "/users",
  account: "/account",
} as const;

export const usersModule: FrontendModule = {
  id: "users",
  protected: [
    { path: USERS_PATHS.list, element: <UsersPage /> },
    { path: USERS_PATHS.account, element: <AccountPage /> },
  ],
  nav: [{ label: "Pengguna", to: USERS_PATHS.list, icon: Users, group: "Pengaturan", order: 900, permission: "users.read" }],
  slots: { "user-menu:items": <ProfileMenuItem /> },
};
