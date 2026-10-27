import { useEffect, useState } from "react";
import { Navigate, Outlet, useLocation, useNavigate } from "react-router-dom";
import axios from "axios";
import { Button } from "@/shared/components/primitives/button";
import { Spinner } from "@/shared/components/primitives/spinner";
import { ErrorState } from "@/shared/components/ui/error-state";
import { Notice } from "@/shared/components/ui/notice";
import { apiErrorMessage } from "@/shared/services/api-error";
import { onUnauthorized } from "@/shared/services/http-client";
import { useSessionStore } from "@/modules/auth/stores/session.store";
import { AUTH_PATHS } from "@/modules/auth/paths";

/** The registry's guard: protected pages render only for a signed-in user. */
export function RequireAuth() {
  const { status, load, expire, signOut } = useSessionStore();
  const location = useLocation();
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);
  // Signed in, but allowed nowhere (403): e.g. not placed in any company or branch yet.
  const [refused, setRefused] = useState(false);

  // Any 401 from the API (session expired, user deactivated) signs out here.
  useEffect(() => onUnauthorized(expire), [expire]);

  useEffect(() => {
    if (status === "unknown" && !error)
      load().catch((err) => {
        setRefused(axios.isAxiosError(err) && err.response?.status === 403);
        setError(apiErrorMessage(err));
      });
  }, [status, load, error]);

  if (status === "signed-in") return <Outlet />;
  if (status === "signed-out") {
    const next = location.pathname + location.search;
    return <Navigate to={`${AUTH_PATHS.login}?next=${encodeURIComponent(next)}`} replace />;
  }
  if (error && refused) {
    return (
      <div className="flex min-h-screen items-center justify-center p-4">
        <div className="grid w-full max-w-md gap-4">
          <Notice tone="warning" title="Anda belum bisa masuk ke aplikasi">
            {error}
          </Notice>
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={() => setError(null)}>
              Coba lagi
            </Button>
            <Button
              onClick={async () => {
                await signOut();
                setError(null);
                navigate(AUTH_PATHS.login, { replace: true });
              }}
            >
              Keluar
            </Button>
          </div>
        </div>
      </div>
    );
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
