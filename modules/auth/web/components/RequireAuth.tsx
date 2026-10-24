import { useEffect, useState } from "react";
import { Navigate, Outlet, useLocation } from "react-router-dom";
import { Spinner } from "@/shared/components/primitives/spinner";
import { ErrorState } from "@/shared/components/ui/error-state";
import { apiErrorMessage } from "@/shared/services/api-error";
import { onUnauthorized } from "@/shared/services/http-client";
import { useSessionStore } from "@/modules/auth/stores/session.store";
import { AUTH_PATHS } from "@/modules/auth/paths";

/** The registry's guard: protected pages render only for a signed-in user. */
export function RequireAuth() {
  const { status, load, expire } = useSessionStore();
  const location = useLocation();
  const [error, setError] = useState<string | null>(null);

  // Any 401 from the API (session expired, user deactivated) signs out here.
  useEffect(() => onUnauthorized(expire), [expire]);

  useEffect(() => {
    if (status === "unknown" && !error) load().catch((err) => setError(apiErrorMessage(err)));
  }, [status, load, error]);

  if (status === "signed-in") return <Outlet />;
  if (status === "signed-out") {
    const next = location.pathname + location.search;
    return <Navigate to={`${AUTH_PATHS.login}?next=${encodeURIComponent(next)}`} replace />;
  }
  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <ErrorState message={error} onRetry={() => setError(null)} />
      </div>
    );
  }
  return (
    <div className="flex min-h-screen items-center justify-center text-muted-foreground">
      <Spinner className="size-5" />
    </div>
  );
}
