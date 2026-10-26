/**
 * App-wide events — how one module tells others something happened without importing them.
 * Add a name here only for something every module may care about.
 */
export type AppEvent =
  /** Someone signed in or out, or the session expired. */
  | "session:changed"
  /** The signed-in user's own profile changed (their name): whoever shows it reloads it. */
  | "profile:changed";

const listeners = new Map<AppEvent, Set<() => void>>();

export function emitAppEvent(event: AppEvent): void {
  listeners.get(event)?.forEach((fn) => fn());
}

/** Runs fn on every event; returns a function that stops it. */
export function onAppEvent(event: AppEvent, fn: () => void): () => void {
  const set = listeners.get(event) ?? new Set();
  set.add(fn);
  listeners.set(event, set);
  return () => {
    set.delete(fn);
  };
}
