import axios from "axios";

export const httpClient = axios.create({
  // Empty = same origin. Proxied by Vite in dev, served by the same container in
  // production. Only set VITE_API_BASE_URL when web and api are deployed separately.
  baseURL: import.meta.env.VITE_API_BASE_URL || "",
  // A separate API origin only receives the session cookie when asked to.
  withCredentials: Boolean(import.meta.env.VITE_API_BASE_URL),
  timeout: 30000,
  headers: { "Content-Type": "application/json" },
});

// Hooks for the auth module, so it plugs in without editing this file.
let authToken: string | null = null;
const unauthorizedHandlers = new Set<() => void>();

/** Sets the bearer token sent with every request; null clears it. */
export function setAuthToken(token: string | null): void {
  authToken = token;
}

/** Runs handler whenever the API answers 401. Returns a function that removes it. */
export function onUnauthorized(handler: () => void): () => void {
  unauthorizedHandlers.add(handler);
  return () => {
    unauthorizedHandlers.delete(handler);
  };
}

httpClient.interceptors.request.use((config) => {
  if (authToken) config.headers.Authorization = `Bearer ${authToken}`;
  return config;
});

httpClient.interceptors.response.use(undefined, (error: unknown) => {
  if (axios.isAxiosError(error) && error.response?.status === 401) {
    unauthorizedHandlers.forEach((handler) => handler());
  }
  return Promise.reject(error);
});
