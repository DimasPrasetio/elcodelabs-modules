import { useId, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Textarea } from "@/shared/components/primitives/textarea";
import { FormField } from "@/shared/components/ui/form-field";
import { Modal } from "@/shared/components/ui/modal";

export interface ReasonDialogProps {
  open: boolean;
  onClose: () => void;
  title: string;
  /** What happens if confirmed (which record, what effect). */
  description: string;
  /** E.g. "Batalkan surat jalan". */
  confirmLabel: string;
  placeholder?: string;
  submitting?: boolean;
  maxLength?: number;
  /** Receives the trimmed reason. */
  onSubmit: (reason: string) => void;
}

/** A confirmation that requires a reason before a corrective action (cancel, void). */
export function ReasonDialog({ open, ...rest }: ReasonDialogProps) {
  // Mounted fresh on every open, so an old reason never lingers.
  return open ? <ReasonDialogBody {...rest} /> : null;
}

function ReasonDialogBody({
  onClose,
  title,
  description,
  confirmLabel,
  placeholder = "Tulis alasan singkat, mis. salah input jumlah",
  submitting = false,
  maxLength = 255,
  onSubmit,
}: Omit<ReasonDialogProps, "open">) {
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string>();
  const formId = useId();

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const r = reason.trim();
    if (r.length < 3) {
      setError(r ? "Alasan minimal 3 karakter" : "Alasan wajib diisi");
      return;
    }
    setError(undefined);
    onSubmit(r);
  }

  return (
    <Modal
      open
      size="sm"
      title={title}
      description={description}
      onClose={submitting ? () => undefined : onClose}
      actions={
        <>
          <Button variant="outline" disabled={submitting} onClick={onClose}>
            Kembali
          </Button>
          <Button type="submit" form={formId} variant="destructive" loading={submitting}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={submit} noValidate>
        <FormField label="Alasan" required error={error}>
          <Textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={3} maxLength={maxLength} placeholder={placeholder} autoFocus />
        </FormField>
      </form>
    </Modal>
  );
}
