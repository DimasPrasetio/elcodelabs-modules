import { useCallback, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { AccessProvider } from "@/shared/access";
import { onAppEvent } from "@/shared/app-events";
import { fetchMyPermissions } from "@/modules/rbac/services/rbac.service";

/**
 * Supplies @/shared/access with the signed-in user's permissions. Reloads whenever the session
 * changes (an app event the auth module emits), so a new user never sees the last one's.
 */
export function RbacAccessProvider({ children }: { children: ReactNode }) {
  const [granted, setGranted] = useState<Set<string> | null>(null);

  const load = useCallback(() => {
    setGranted(null);
    fetchMyPermissions()
      .then((p) => setGranted(new Set(p)))
      // Not signed in (401) or unreachable: nothing is granted until the next session change.
      .catch(() => setGranted(new Set()));
  }, []);

  useEffect(() => {
    load();
    return onAppEvent("session:changed", load);
  }, [load]);

  const value = useMemo(
    () => ({ can: (permission: string) => granted?.has(permission) ?? false, ready: granted !== null }),
    [granted],
  );
  return <AccessProvider value={value}>{children}</AccessProvider>;
}
