import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { ZONES } from "@/modules/organization/types/organization";

interface ZoneSelectProps {
  id?: string;
  value: string;
  onChange: (zone: string) => void;
  /** Offer "the company's" (an empty value), for a branch. */
  inherit?: boolean;
}

/** WIB, WITA, or WIT — and a zone stored otherwise, shown as it is. */
export function ZoneSelect({ id, value, onChange, inherit }: ZoneSelectProps) {
  const known = value === "" || ZONES.some((z) => z.value === value);
  return (
    <NativeSelect id={id} value={value} onChange={(e) => onChange(e.target.value)}>
      {inherit && <NativeSelectOption value="">Ikut zona perusahaan</NativeSelectOption>}
      {ZONES.map((z) => (
        <NativeSelectOption key={z.value} value={z.value}>
          {z.label}
        </NativeSelectOption>
      ))}
      {!known && <NativeSelectOption value={value}>{value}</NativeSelectOption>}
    </NativeSelect>
  );
}
