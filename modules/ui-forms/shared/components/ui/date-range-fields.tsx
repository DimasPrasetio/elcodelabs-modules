import { Input } from "@/shared/components/primitives/input";
import { FormField } from "@/shared/components/ui/form-field";

export interface DateRange {
  from: string;
  to: string;
}

export interface DateRangeFieldsProps extends DateRange {
  onChange: (range: DateRange) => void;
  fromLabel?: string;
  toLabel?: string;
}

/** Two date fields (YYYY-MM-DD) that limit each other and say so when reversed. */
export function DateRangeFields({ from, to, onChange, fromLabel = "Dari tanggal", toLabel = "Sampai tanggal" }: DateRangeFieldsProps) {
  const reversed = Boolean(from && to && from > to);
  return (
    <>
      <FormField label={fromLabel} error={reversed ? "Tanggal awal melewati tanggal akhir" : undefined}>
        <Input type="date" value={from} max={to || undefined} onChange={(e) => onChange({ from: e.target.value, to })} />
      </FormField>
      <FormField label={toLabel}>
        <Input type="date" value={to} min={from || undefined} onChange={(e) => onChange({ from, to: e.target.value })} />
      </FormField>
    </>
  );
}
