import { useState } from "react";
import type { ComponentProps } from "react";
import { cn } from "cn";
import { FIELD } from "@/shared/lib/field-classes";

export interface DigitsInputProps extends Omit<ComponentProps<"input">, "value" | "onChange" | "type" | "inputMode"> {
  /** Digits only. */
  value: string;
  onChange: (digits: string) => void;
  /** Most digits accepted; extra ones are dropped. */
  maxDigits?: number;
  /** Group sizes shown while the field is not focused, e.g. [4, 4, 4, 4] → "3201 2345 6789 0001". */
  groups?: number[];
}

/** groupDigits shows digits in groups of the given sizes; what is left over forms the last group. */
export function groupDigits(digits: string, groups: number[]): string {
  const parts: string[] = [];
  let at = 0;
  for (const size of groups) {
    if (at >= digits.length) break;
    parts.push(digits.slice(at, at + size));
    at += size;
  }
  if (at < digits.length) parts.push(digits.slice(at));
  return parts.join(" ");
}

/**
 * A number typed as text — an ID, an account, a code where leading zeros count. Anything but
 * digits is dropped as it is typed or pasted; grouping is shown only while not editing, so the
 * cursor never jumps.
 */
export function DigitsInput({ value, onChange, maxDigits, groups, className, onFocus, onBlur, ...props }: DigitsInputProps) {
  const [focused, setFocused] = useState(false);
  const shown = !focused && groups?.length ? groupDigits(value, groups) : value;
  return (
    <input
      {...props}
      type="text"
      inputMode="numeric"
      autoComplete="off"
      value={shown}
      onFocus={(e) => {
        setFocused(true);
        onFocus?.(e);
      }}
      onBlur={(e) => {
        setFocused(false);
        onBlur?.(e);
      }}
      onChange={(e) => {
        const digits = e.target.value.replace(/\D/g, "");
        onChange(maxDigits ? digits.slice(0, maxDigits) : digits);
      }}
      className={cn(FIELD, "font-mono tabular-nums", className)}
    />
  );
}
