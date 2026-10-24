import { useState } from "react";
import {
  SelectContent,
  SelectList,
  SelectMessage,
  SelectOptionButton,
  SelectRoot,
  SelectSearch,
  SelectTrigger,
  optionSearchText,
} from "@/shared/components/ui/select-parts";
import type { SelectOption, SelectValue } from "@/shared/components/ui/select-parts";

export type { SelectOption, SelectValue };

export interface SearchSelectProps {
  options: SelectOption[];
  value: SelectValue | null | undefined;
  onChange: (value: SelectValue | null, option?: SelectOption) => void;
  placeholder?: string;
  searchPlaceholder?: string;
  emptyText?: string;
  /** Shows × to clear the choice. */
  allowClear?: boolean;
  disabled?: boolean;
  id?: string;
  /** Submits the value with a plain <form> (hidden input). */
  name?: string;
  "aria-label"?: string;
  "aria-invalid"?: boolean;
  "aria-describedby"?: string;
  className?: string;
}

/** A select over options known up front, with a search box. For server-side search: AsyncSearchSelect. */
export function SearchSelect({
  options,
  value,
  onChange,
  placeholder = "Pilih…",
  searchPlaceholder = "Cari…",
  emptyText = "Tidak ada hasil",
  allowClear = false,
  disabled,
  id,
  name,
  className,
  ...aria
}: SearchSelectProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const selected = options.find((o) => o.value === value);
  const q = query.trim().toLowerCase();
  const filtered = q ? options.filter((o) => optionSearchText(o).includes(q)) : options;
  const hasValue = value !== null && value !== undefined && value !== "";

  function choose(o: SelectOption) {
    onChange(o.value, o);
    setOpen(false);
  }

  return (
    <SelectRoot
      open={open && !disabled}
      onOpenChange={(next) => {
        if (next) setQuery("");
        setOpen(next);
      }}
    >
      {name && <input type="hidden" name={name} value={hasValue ? String(value) : ""} />}
      <SelectTrigger
        id={id}
        disabled={disabled}
        label={selected ? selected.label : placeholder}
        empty={!selected}
        open={open}
        onClear={allowClear && hasValue && !disabled ? () => onChange(null) : undefined}
        className={className}
        {...aria}
      />
      <SelectContent>
        <SelectSearch value={query} onChange={(e) => setQuery(e.target.value)} placeholder={searchPlaceholder} aria-label={searchPlaceholder} />
        <SelectList>
          {filtered.length === 0 ? (
            <SelectMessage>{emptyText}</SelectMessage>
          ) : (
            filtered.map((o) => <SelectOptionButton key={o.value} option={o} active={o.value === value} onChoose={() => choose(o)} />)
          )}
        </SelectList>
      </SelectContent>
    </SelectRoot>
  );
}
