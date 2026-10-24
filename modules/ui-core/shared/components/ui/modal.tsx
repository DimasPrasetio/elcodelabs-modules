import type { ReactNode } from "react";
import { cn } from "cn";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/components/primitives/dialog";

export type ModalSize = "sm" | "md" | "lg" | "xl";

const MAX_WIDTH: Record<ModalSize, string> = {
  sm: "sm:max-w-[440px]",
  md: "sm:max-w-[560px]",
  lg: "sm:max-w-[760px]",
  xl: "sm:max-w-[960px]",
};

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  /** A badge beside the title. */
  badge?: ReactNode;
  /** Footer buttons, main action last. */
  actions?: ReactNode;
  size?: ModalSize;
  children: ReactNode;
}

/**
 * A dialog for a short task — up to about six fields in one step. Focus is trapped and
 * restored, Esc and the overlay close it. Longer forms and master-data editing use FormSheet.
 */
export function Modal({ open, onClose, title, description, badge, actions, size = "md", children }: ModalProps) {
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className={cn("block", MAX_WIDTH[size])}>
        <DialogHeader className="pr-8">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <DialogTitle>{title}</DialogTitle>
            {badge}
          </div>
          <DialogDescription className={description ? undefined : "sr-only"}>{description ?? title}</DialogDescription>
        </DialogHeader>
        <div className="mt-4">{children}</div>
        {actions && <DialogFooter className="mt-6">{actions}</DialogFooter>}
      </DialogContent>
    </Dialog>
  );
}
