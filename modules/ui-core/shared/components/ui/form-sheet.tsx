import type { ReactNode } from "react";
import { cn } from "cn";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/shared/components/primitives/sheet";

export type FormSheetSize = "md" | "lg";

const WIDTH: Record<FormSheetSize, string> = { md: "sm:max-w-[480px]", lg: "sm:max-w-[640px]" };

export interface FormSheetProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  badge?: ReactNode;
  /** Footer buttons (Batal · Simpan), pinned to the bottom of the panel. */
  actions?: ReactNode;
  size?: FormSheetSize;
  children: ReactNode;
}

/**
 * A right-side panel for creating and editing master data and for actions on a record. Same API
 * as Modal, so switching between them is a rename.
 */
export function FormSheet({ open, onClose, title, description, badge, actions, size = "md", children }: FormSheetProps) {
  return (
    <Sheet open={open} onOpenChange={(next) => !next && onClose()}>
      <SheetContent side="right" className={cn("flex w-full flex-col gap-0 p-0", WIDTH[size])}>
        <SheetHeader className="border-b border-border p-5 text-left">
          <div className="flex flex-wrap items-center gap-2 pr-8">
            <SheetTitle className="text-xl font-semibold text-foreground">{title}</SheetTitle>
            {badge}
          </div>
          <SheetDescription className={description ? "text-sm text-muted-foreground" : "sr-only"}>{description ?? title}</SheetDescription>
        </SheetHeader>
        <div className="flex-1 overflow-y-auto p-5">{children}</div>
        {actions && <SheetFooter className="flex-row justify-end gap-2 border-t border-border bg-card p-4">{actions}</SheetFooter>}
      </SheetContent>
    </Sheet>
  );
}
