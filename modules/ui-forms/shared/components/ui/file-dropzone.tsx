import { useId, useRef, useState } from "react";
import type { DragEvent } from "react";
import { FileUp } from "lucide-react";
import { cn } from "cn";

export interface FileDropzoneProps {
  /** Receives the chosen file; uploading is the caller's job. */
  onFile: (file: File) => void;
  accept?: string;
  disabled?: boolean;
  busy?: boolean;
  label?: string;
  busyLabel?: string;
  hint?: string;
  /** The chosen file's name, shown instead of the hint. */
  fileName?: string | null;
  className?: string;
}

/** The one file input: drag and drop, or click to choose. */
export function FileDropzone({
  onFile,
  accept,
  disabled = false,
  busy = false,
  label = "Seret berkas ke sini atau pilih",
  busyLabel = "Mengunggah…",
  hint,
  fileName,
  className,
}: FileDropzoneProps) {
  const id = useId();
  const input = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);
  const off = disabled || busy;

  function pick(files: FileList | null) {
    const f = files?.[0];
    if (f && !off) onFile(f);
    if (input.current) input.current.value = "";
  }

  function drop(e: DragEvent<HTMLLabelElement>) {
    e.preventDefault();
    setOver(false);
    pick(e.dataTransfer.files);
  }

  return (
    <label
      htmlFor={id}
      onDragOver={(e) => {
        e.preventDefault();
        if (!off) setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={drop}
      className={cn(
        "flex cursor-pointer flex-col items-center gap-1 rounded-lg border-2 border-dashed border-input bg-card px-4 py-6 text-center transition-colors hover:bg-background",
        over && "border-primary bg-accent",
        off && "cursor-not-allowed opacity-60",
        className,
      )}
    >
      <FileUp size={20} aria-hidden className="text-muted-foreground" />
      <span className="text-sm font-medium text-foreground">{busy ? busyLabel : label}</span>
      {fileName ? <span className="max-w-full truncate text-xs text-foreground">{fileName}</span> : hint ? <span className="text-xs text-muted-foreground">{hint}</span> : null}
      <input ref={input} id={id} type="file" className="sr-only" accept={accept} disabled={off} onChange={(e) => pick(e.target.files)} />
    </label>
  );
}
