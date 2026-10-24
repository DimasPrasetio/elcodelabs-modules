// id-ID formatting in one place: rupiah without decimals, Indonesian dates. Components never
// format by hand. An empty or invalid value shows "–".

const LOCALE = "id-ID";
const EMPTY = "–";

/** "Rp 12.450.000" */
export function formatIDR(n: number | null | undefined): string {
  if (n === null || n === undefined || Number.isNaN(n)) return EMPTY;
  return new Intl.NumberFormat(LOCALE, { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(n);
}

/** "1.250,5" */
export function formatNumber(n: number | null | undefined, decimals?: number): string {
  if (n === null || n === undefined || Number.isNaN(n)) return EMPTY;
  return new Intl.NumberFormat(LOCALE, decimals === undefined ? undefined : { minimumFractionDigits: decimals, maximumFractionDigits: decimals }).format(n);
}

// "YYYY-MM-DD" is a calendar date (local midnight); "YYYY-MM-DD hh:mm:ss" from a server is
// made parseable everywhere by turning the space into "T".
function toDate(v: string): Date {
  if (v.length <= 10) return new Date(`${v}T00:00:00`);
  return new Date(v.includes(" ") && !v.includes("T") ? v.replace(" ", "T") : v);
}

/** "10 Sep 2026" */
export function formatDate(v: string | Date | null | undefined): string {
  if (!v) return EMPTY;
  const d = typeof v === "string" ? toDate(v) : v;
  if (Number.isNaN(d.getTime())) return EMPTY;
  return d.toLocaleDateString(LOCALE, { day: "numeric", month: "short", year: "numeric" });
}

/** "10 Sep 2026 14:30" */
export function formatDateTime(v: string | Date | null | undefined): string {
  if (!v) return EMPTY;
  const d = typeof v === "string" ? toDate(v) : v;
  if (Number.isNaN(d.getTime())) return EMPTY;
  return `${formatDate(d)} ${d.toLocaleTimeString(LOCALE, { hour: "2-digit", minute: "2-digit" }).replace(".", ":")}`;
}

/** Today as "YYYY-MM-DD" in a time zone (default WIB) — for date filters. */
export function todayISO(timeZone = "Asia/Jakarta"): string {
  return new Intl.DateTimeFormat("en-CA", { timeZone, year: "numeric", month: "2-digit", day: "2-digit" }).format(new Date());
}
