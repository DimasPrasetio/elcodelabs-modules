import { cloneElement, isValidElement, useId } from "react";
import type { ReactElement, ReactNode } from "react";
import { Label } from "@/shared/components/primitives/label";

export interface FormFieldProps {
  label: string;
  /** One input-like element; it receives id, aria-invalid, and aria-describedby. */
  children: ReactElement<{ id?: string; "aria-invalid"?: boolean; "aria-describedby"?: string }>;
  error?: string;
  hint?: ReactNode;
}

/** Label, control, and its hint or error — wired for screen readers. */
export function FormField({ label, children, error, hint }: FormFieldProps) {
  const id = useId();
  const messageId = `${id}-message`;
  const control = isValidElement(children)
    ? cloneElement(children, {
        id,
        "aria-invalid": error ? true : undefined,
        "aria-describedby": error || hint ? messageId : undefined,
      })
    : children;
  return (
    <div className="grid gap-2">
      <Label htmlFor={id}>{label}</Label>
      {control}
      {error ? (
        <p id={messageId} className="text-sm text-destructive">
          {error}
        </p>
      ) : hint ? (
        <p id={messageId} className="text-sm text-muted-foreground">
          {hint}
        </p>
      ) : null}
    </div>
  );
}
