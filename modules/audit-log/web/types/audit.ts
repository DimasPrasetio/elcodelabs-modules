export interface AuditEntry {
  id: number;
  occurred_at: string;
  actor_id: number;
  actor_name: string;
  module: string;
  action: string;
  target: string;
  summary: string;
  data: Record<string, unknown> | null;
  ip: string;
  request_id: string;
}
