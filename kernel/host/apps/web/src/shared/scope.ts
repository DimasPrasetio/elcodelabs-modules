import { useEffect, useState } from "react";
import { emitAppEvent, onAppEvent } from "@/shared/app-events";

/**
 * The company and branch this tab works in — "<company>" or "<company>:<branch>", empty for the
 * user's default. Sent with every request as X-Scope; the API validates it against where the
 * user is placed. Set by an organization module's picker (slot "shell:scope"); an app with one
 * company never sets it.
 */
const KEY = "elco.scope";

function read(): string {
  try {
    return sessionStorage.getItem(KEY) ?? "";
  } catch {
    return "";
  }
}

export function getScope(): string {
  return read();
}

/** Switches this tab to scope and tells every page to reload ("scope:changed"). */
export function setScope(scope: string): void {
  try {
    if (scope) sessionStorage.setItem(KEY, scope);
    else sessionStorage.removeItem(KEY);
  } catch {
    // Storage blocked: the scope lasts until the page reloads.
  }
  emitAppEvent("scope:changed");
}

/** The current scope; the component re-renders when it changes. */
export function useScope(): string {
  const [scope, set] = useState(read);
  useEffect(() => onAppEvent("scope:changed", () => set(read())), []);
  return scope;
}
