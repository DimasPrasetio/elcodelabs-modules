import { StatusBadge } from "@/shared/components/ui/status-badge";
import type { Supplier } from "@/modules/suppliers/types/supplier";

/** Nonaktif, Ditahan, or Aktif — in that order of weight. */
export function SupplierStatusBadge({ supplier }: { supplier: Pick<Supplier, "is_active" | "on_hold"> }) {
  if (!supplier.is_active) return <StatusBadge tone="muted">Nonaktif</StatusBadge>;
  if (supplier.on_hold) return <StatusBadge tone="warning">Ditahan</StatusBadge>;
  return <StatusBadge tone="success">Aktif</StatusBadge>;
}
