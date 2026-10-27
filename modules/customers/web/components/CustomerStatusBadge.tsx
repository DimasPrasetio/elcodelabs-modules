import { StatusBadge } from "@/shared/components/ui/status-badge";
import type { Customer } from "@/modules/customers/types/customer";

/** Nonaktif, Ditahan, or Aktif — in that order of weight. */
export function CustomerStatusBadge({ customer }: { customer: Pick<Customer, "is_active" | "on_hold"> }) {
  if (!customer.is_active) return <StatusBadge tone="muted">Nonaktif</StatusBadge>;
  if (customer.on_hold) return <StatusBadge tone="warning">Ditahan</StatusBadge>;
  return <StatusBadge tone="success">Aktif</StatusBadge>;
}
