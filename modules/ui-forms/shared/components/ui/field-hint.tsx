import type { ReactNode } from "react";
import { Popover, PopoverContent, PopoverTrigger } from "@/shared/components/primitives/popover";

/** A small "?" beside a label that explains a field's rule without taking room. */
export function FieldHint({ children, label = "Keterangan" }: { children: ReactNode; label?: string }) {
  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={label}
          className="ml-1 inline-flex size-5 shrink-0 items-center justify-center rounded-full bg-muted-foreground/65 text-2xs leading-none font-bold text-primary-foreground transition-colors hover:bg-primary data-[state=open]:bg-primary"
        >
          ?
        </button>
      </PopoverTrigger>
      <PopoverContent side="top" className="w-auto max-w-[260px] px-3 py-2 text-xs leading-normal">
        {children}
      </PopoverContent>
    </Popover>
  );
}
