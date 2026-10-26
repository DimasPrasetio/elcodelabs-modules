export type MailStatus = "pending" | "sending" | "sent" | "failed";

export interface OutboxMessage {
  id: number;
  to: string;
  to_name: string;
  subject: string;
  status: MailStatus;
  attempts: number;
  next_attempt_at: string;
  last_error: string;
  created_at: string;
  sent_at: string | null;
}
