export const RBAC_PATHS = {
  roles: "/roles",
  role: (id: number) => `/roles/${id}`,
} as const;
