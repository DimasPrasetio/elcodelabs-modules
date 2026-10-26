import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import axios from "axios";
import type { ReactNode } from "react";
import { AccessProvider } from "@/shared/access";
import { onAppEvent } from "@/shared/app-events";
import { fetchMyPermissions } from "@/modules/rbac/services/rbac.service";

const RETRY_MS = [1000, 3000, 8000];

/**
 * Supplies @/shared/access with the signed-in user's permissions. Reloads whenever the session
 * changes (an app event the auth module emits), so a new user never sees the last one's: an
 * answer that arrives after a newer request started is dropped. A network failure is retried a
 * few times before giving up (ready, nothing granted); not being signed in (401) is final.
 */
export function RbacAccessProvider({ children }: { children: ReactNode }) {
  const [granted, setGranted] = useState<Set<string> | null>(null);
  const request = useRef(0);
  const timer = useRef<number | null>(null);

  const load = useCallback(() => {
    const id = ++request.current;
    if (timer.current !== null) window.clearTimeout(timer.current);
    setGranted(null);
    const attempt = (n: number) => {
      fetchMyPermissions()
        .then((p) => id === request.current && setGranted(new Set(p)))
        .catch((err: unknown) => {
          if (id !== request.current) return;
          const status = axios.isAxiosError(err) ? err.response?.status : undefined;
          if (status === undefined || status >= 500) {
            if (n < RETRY_MS.length) {
              timer.current = window.setTimeout(() => attempt(n + 1), RETRY_MS[n]);
              return;
            }
          }
          setGranted(new Set());
        });
    };
    attempt(0);
  }, []);

  useEffect(() => {
    load();
    const off = onAppEvent("session:changed", load);
    return () => {
      off();
      request.current++;
      if (timer.current !== null) window.clearTimeout(timer.current);
    };
  }, [load]);

  const value = useMemo(
    () => ({ can: (permission: string) => granted?.has(permission) ?? false, ready: granted !== null }),
    [granted],
  );
  return <AccessProvider value={value}>{children}</AccessProvider>;
}
