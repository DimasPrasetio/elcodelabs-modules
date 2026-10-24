import { cloneElement, isValidElement, useId } from "react";
import type { ReactElement, ReactNode } from "react";

export interface FormFieldProps {
  label: string;
  /** One control; it gets id, aria-invalid, and aria-describedby wired. */
  children: ReactElement<{ id?: string; "aria-invalid"?: boolean; "aria-describedby"?: string }> | ReactNode;
  required?: boolean;
  /** Help under the control. */
  hint?: ReactNode;
  /** Replaces the hint while set. */
  error?: string;
  /** For a control that is not the direct child (e.g. a composite). */
  htmlFor?: string;
  /** Extra beside the label, e.g. a FieldHint. */
  labelAside?: ReactNode;
}

/** Label, control, and its hint or error. Required is a calm "*", read as "wajib diisi". */
export function FormField({ label, children, required, hint, error, htmlFor, labelAside }: FormFieldProps) {
  const autoId = useId();
  const messageId = `${autoId}-message`;
  const single = isValidElement(children) ? (children as ReactElement<{ id?: string }>) : null;
  const controlId = htmlFor ?? single?.props.id ?? (single ? autoId : undefined);
  const control = single
    ? cloneElement(single as ReactElement<Record<string, unknown>>, {
        id: controlId,
        "aria-invalid": error ? true : undefined,
        "aria-describedby": error || hint ? messageId : undefined,
      })
    : children;

  return (
    <div className="min-w-0">
      <label htmlFor={controlId} className="mb-1.5 flex items-center gap-1 text-sm font-medium text-foreground">
        {label}
        {required && (
          <>
            <span aria-hidden className="text-destructive">
              *
            </span>
            <span className="sr-only">(wajib diisi)</span>
          </>
        )}
        {labelAside}
      </label>
      {control}
      {error ? (
        <p id={messageId} role="alert" className="mt-1.5 text-xs font-medium text-destructive">
          {error}
        </p>
      ) : hint ? (
        <p id={messageId} className="mt-1.5 text-xs text-muted-foreground">
          {hint}
        </p>
      ) : null}
    </div>
  );
}
