import { useState } from "react";
import type { ReactNode } from "react";
import { Alert, AlertDescription } from "@/shared/components/primitives/alert";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/shared/components/primitives/alert-dialog";
import { Button } from "@/shared/components/primitives/button";
import { apiErrorMessage } from "@/shared/services/api-error";

export interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  /** "danger" for something that cannot be undone. */
  tone?: "danger" | "primary";
  /**
   * May be async: the dialog stays busy until it settles and closes on success. If it throws,
   * the dialog stays open and shows why (the API's message when there is one).
   */
  onConfirm: () => void | Promise<void>;
}

/** Asks before an action that is hard to undo. "Batal" has focus, so Enter is the safe choice. */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel = "Konfirmasi",
  cancelLabel = "Batal",
  tone = "danger",
  onConfirm,
}: ConfirmDialogProps) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function confirm() {
    setBusy(true);
    setError(null);
    try {
      await onConfirm();
      onOpenChange(false);
    } catch (err) {
      setError(apiErrorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <AlertDialog
      open={open}
      onOpenChange={(next) => {
        if (busy) return;
        if (!next) setError(null);
        onOpenChange(next);
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          {description && <AlertDialogDescription className="text-base text-foreground">{description}</AlertDialogDescription>}
        </AlertDialogHeader>
        {error && (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel asChild>
            <Button variant="outline" size="sm" disabled={busy}>
              {cancelLabel}
            </Button>
          </AlertDialogCancel>
          <Button variant={tone === "danger" ? "destructive" : "default"} size="sm" loading={busy} onClick={confirm}>
            {confirmLabel}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
