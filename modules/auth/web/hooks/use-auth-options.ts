import { useEffect, useState } from "react";
import { fetchOptions } from "@/modules/auth/services/auth.service";
import type { AuthOptions } from "@/modules/auth/services/auth.service";

let cached: Promise<AuthOptions> | null = null;

/** Which password features this install offers (they depend on the modules wired to auth). */
export function useAuthOptions(): AuthOptions {
  const [options, setOptions] = useState<AuthOptions>({ password_change: false, password_reset: false });
  useEffect(() => {
    let alive = true;
    cached ??= fetchOptions().catch(() => {
      cached = null;
      return { password_change: false, password_reset: false };
    });
    void cached.then((o) => alive && setOptions(o));
    return () => {
      alive = false;
    };
  }, []);
  return options;
}
