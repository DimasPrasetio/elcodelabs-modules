import { createContext, useContext } from "react";

/** What the signed-in user may do. Permissions are "<module>.<action>", e.g. "users.manage". */
export interface Access {
  can: (permission: string) => boolean;
  /** False while the provider is still loading the user's permissions. */
  ready: boolean;
}

// Without an authorization module everything is allowed — the backend's default policy is
// "signed in = allowed" too. An rbac module provides the real answer.
const AccessContext = createContext<Access>({ can: () => true, ready: true });

/** Provided by the module that does authorization. */
export const AccessProvider = AccessContext.Provider;

export function useAccess(): Access {
  return useContext(AccessContext);
}

/** Whether to show what needs permission. The API enforces it either way. */
export function useCan(permission: string): boolean {
  return useAccess().can(permission);
}
