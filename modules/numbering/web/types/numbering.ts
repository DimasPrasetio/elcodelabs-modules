export type Reset = "never" | "yearly" | "monthly" | "daily";

export interface NumberFormat {
  pattern: string;
  reset: Reset;
  per_branch: boolean;
}

export interface SeriesPreview {
  /** 0: the company-wide series. */
  branch_id: number;
  branch_name: string;
  last_value: number;
  next: string;
  /** The user may raise this series' next number. */
  manageable: boolean;
}

export interface DocType {
  doc_type: string;
  label: string;
  format: NumberFormat;
  default: NumberFormat;
  /** The company changed the format from the module's default. */
  custom: boolean;
  updated_at: string | null;
  /** The series within the user's reach (a branch reader sees their branch only). */
  series: SeriesPreview[];
}

export interface NumberingOverview {
  types: DocType[];
  today: string;
  /** numbering.manage across the whole company — a role in one branch does not count. */
  manage_formats: boolean;
}
