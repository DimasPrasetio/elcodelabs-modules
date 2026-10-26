export type AuditKind = "change" | "access" | "export" | "security";

export interface AuditEntry {
  id: number;
  occurred_at: string;
  actor_id: number;
  actor_name: string;
  /** change · access (personal data viewed) · export · security */
  kind: AuditKind;
  /** 0: the company itself. */
  branch_id: number;
  module: string;
  action: string;
  target: string;
  summary: string;
  data: Record<string, unknown> | null;
  ip: string;
  request_id: string;
}
