import { useCallback, useEffect, useRef, useState } from "react";
import type { DependencyList } from "react";
import { apiErrorMessage } from "@/shared/services/api-error";

export interface Resource<T> {
  /** The latest answer; kept while a newer request loads, so a list does not flash empty. */
  data: T | undefined;
  loading: boolean;
  /** A readable message, or null. */
  error: string | null;
  /** Loads again with the same inputs (a retry button, after saving). */
  reload: () => void;
}

/**
 * Loads what a page shows and keeps only the latest answer. When deps change, the previous
 * request is aborted through its signal and anything it still returns is dropped — so page 2
 * of an old search can never replace page 1 of the new one.
 *
 *   const users = useResource((signal) => listUsers({ page, q }, signal), [page, q]);
 */
export function useResource<T>(load: (signal: AbortSignal) => Promise<T>, deps: DependencyList): Resource<T> {
  const [data, setData] = useState<T>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [round, setRound] = useState(0);
  const latest = useRef(load);
  latest.current = load;

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    latest
      .current(controller.signal)
      .then((value) => {
        if (!controller.signal.aborted) setData(value);
      })
      .catch((err: unknown) => {
        if (!controller.signal.aborted) setError(apiErrorMessage(err));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
    // The caller's deps decide when to load again; `load` itself changes on every render.
  }, [...deps, round]); // eslint-disable-line react-hooks/exhaustive-deps

  const reload = useCallback(() => setRound((n) => n + 1), []);
  return { data, loading, error, reload };
}
