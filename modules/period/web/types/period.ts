export interface AreaLock {
  area: string;
  label: string;
  /** false: a lock kept for an area no installed module uses any more. */
  declared: boolean;
  own: { locked_through: string; updated_by: number; updated_at: string } | null;
  /** What actually closes the area (YYYY-MM-DD): the later of its own and "*"; null when open. */
  effective: string | null;
}

export interface PeriodOverview {
  areas: AreaLock[];
  fiscal_year_start_month: number;
  /** Today in the company's time zone (YYYY-MM-DD). */
  today: string;
  /** period.manage across the whole company — a role in one branch does not count. */
  can_close: boolean;
  /** period.unlock across the whole company. */
  can_reopen: boolean;
}
