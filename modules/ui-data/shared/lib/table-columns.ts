// Which columns a user hid in a DataTable, remembered per table in localStorage. Storage may be
// unavailable (private mode, blocked): every access is guarded and falls back to the defaults.

const PREFIX = "elco.table-columns.";
const EVENT = "elco:table-columns";

export function readHiddenColumns(key: string): string[] | null {
  try {
    const raw = window.localStorage.getItem(PREFIX + key);
    const parsed: unknown = raw ? JSON.parse(raw) : null;
    return Array.isArray(parsed) ? parsed.filter((x): x is string => typeof x === "string") : null;
  } catch {
    return null;
  }
}

export function writeHiddenColumns(key: string, hidden: string[]): void {
  try {
    window.localStorage.setItem(PREFIX + key, JSON.stringify(hidden));
  } catch {
    // Not remembered — the choice still applies until the page reloads.
  }
  window.dispatchEvent(new CustomEvent(EVENT, { detail: key }));
}

/** Runs fn when another part of the page changes the columns of table key. */
export function onHiddenColumnsChange(key: string, fn: () => void): () => void {
  const handler = (e: Event) => {
    if ((e as CustomEvent<string>).detail === key) fn();
  };
  window.addEventListener(EVENT, handler);
  return () => window.removeEventListener(EVENT, handler);
}
