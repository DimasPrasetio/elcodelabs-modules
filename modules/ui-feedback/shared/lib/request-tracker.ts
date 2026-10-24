import { useSyncExternalStore } from "react";
import { httpClient } from "@/shared/services/http-client";

// Counts the http-client requests in flight, for the global loading bar. Interceptors are added
// once, the first time anything subscribes.

let pending = 0;
let attached = false;
const listeners = new Set<() => void>();

function set(next: number) {
  pending = Math.max(0, next);
  listeners.forEach((fn) => fn());
}

function attach() {
  if (attached) return;
  attached = true;
  httpClient.interceptors.request.use((config) => {
    set(pending + 1);
    return config;
  });
  httpClient.interceptors.response.use(
    (res) => {
      set(pending - 1);
      return res;
    },
    (err: unknown) => {
      set(pending - 1);
      return Promise.reject(err);
    },
  );
}

function subscribe(fn: () => void) {
  attach();
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}

/** True while at least one request is running. */
export function useRequestsPending(): boolean {
  return useSyncExternalStore(subscribe, () => pending > 0, () => false);
}
