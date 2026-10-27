import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import axios from "axios";
import { onAppEvent } from "@/shared/app-events";
import { getScope, setScope } from "@/shared/scope";
import { Spinner } from "@/shared/components/primitives/spinner";
import { fetchMe } from "@/modules/organization/services/organization.service";
import type { Me } from "@/modules/organization/types/organization";

interface OrganizationState {
  /** Where the signed-in user may work, and where this tab works now; null when signed out. */
  me: Me | null;
  reload: () => void;
}

const OrganizationContext = createContext<OrganizationState>({ me: null, reload: () => undefined });

export function useOrganization(): OrganizationState {
  return useContext(OrganizationContext);
}

/**
 * Keeps the tab's company/branch (X-Scope) valid. A scope stored in an earlier visit may have
 * gone stale — the company deactivated, the placement taken away — and the API then refuses
 * every request (403): before the app renders, such a scope is checked once and dropped, so the
 * tab falls back to the user's default. Signing out forgets it too. /me needs no permission, so
 * a 403 from it is always about the scope.
 */
export function OrganizationProvider({ children }: { children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [checked, setChecked] = useState(() => getScope() === "");
  const request = useRef(0);

  const load = useCallback(() => {
    const id = ++request.current;
    fetchMe()
      .then((m) => {
        if (id !== request.current) return;
        setMe(m);
        setChecked(true);
      })
      .catch((err: unknown) => {
        if (id !== request.current) return;
        const status = axios.isAxiosError(err) ? err.response?.status : undefined;
        if ((status === 403 || status === 401) && getScope() !== "") {
          setScope(""); // announces scope:changed, which loads again
          return;
        }
        // Signed out, placed nowhere (the sign-in guard says so), or the API is down.
        setMe(null);
        setChecked(true);
      });
  }, []);

  useEffect(() => {
    load();
    const offSession = onAppEvent("session:changed", load);
    const offScope = onAppEvent("scope:changed", load);
    return () => {
      offSession();
      offScope();
      request.current++;
    };
  }, [load]);

  const value = useMemo(() => ({ me, reload: load }), [me, load]);
  if (!checked) {
    return (
      <div className="flex min-h-screen items-center justify-center text-muted-foreground">
        <Spinner className="size-5" />
      </div>
    );
  }
  return <OrganizationContext.Provider value={value}>{children}</OrganizationContext.Provider>;
}
