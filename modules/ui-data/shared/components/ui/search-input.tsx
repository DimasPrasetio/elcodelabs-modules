import { useEffect, useRef, useState } from "react";
import { Search } from "lucide-react";
import { FIELD } from "@/shared/lib/field-classes";

export interface SearchInputProps {
  /** The applied value (e.g. from the URL). Changing it from outside resets the text. */
  value: string;
  /** Called after typing pauses, or on Enter or blur — never per keystroke. */
  onSearch: (value: string) => void;
  placeholder?: string;
  debounceMs?: number;
  id?: string;
  disabled?: boolean;
  "aria-label"?: string;
}

/** A search box that reports only settled text, so typing does not fire a request per key. */
export function SearchInput({ value, onSearch, placeholder = "Cari…", debounceMs = 400, id, disabled, "aria-label": ariaLabel = "Cari" }: SearchInputProps) {
  const [text, setText] = useState(value);
  const [prev, setPrev] = useState(value);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const applied = useRef(value);

  // Follow an outside change (reset) unless it is what is being typed.
  if (value !== prev) {
    setPrev(value);
    if (value !== text.trim()) setText(value);
  }

  useEffect(() => {
    applied.current = value;
  }, [value]);

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  function emit(next: string) {
    const trimmed = next.trim();
    if (trimmed === applied.current) return;
    applied.current = trimmed;
    onSearch(trimmed);
  }

  function flush() {
    if (timer.current) clearTimeout(timer.current);
    emit(text);
  }

  return (
    <div className="relative">
      <Search aria-hidden className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
      <input
        id={id}
        type="search"
        aria-label={ariaLabel}
        disabled={disabled}
        className={`${FIELD} pl-9`}
        value={text}
        placeholder={placeholder}
        onChange={(e) => {
          setText(e.target.value);
          if (timer.current) clearTimeout(timer.current);
          const next = e.target.value;
          timer.current = setTimeout(() => emit(next), debounceMs);
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            flush();
          }
        }}
        onBlur={flush}
      />
    </div>
  );
}
