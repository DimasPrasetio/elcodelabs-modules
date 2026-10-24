import { Button } from "@/shared/components/primitives/button";
import { Notice } from "@/shared/components/ui/notice";

export interface ErrorStateProps {
  /** The API's own message — never a generic one when the API said why. */
  message: string;
  onRetry?: () => void;
  title?: string;
  retryLabel?: string;
  className?: string;
}

/** A failed load: what went wrong, and "Coba lagi". */
export function ErrorState({ message, onRetry, title = "Gagal memuat", retryLabel = "Coba lagi", className }: ErrorStateProps) {
  return (
    <Notice tone="danger" title={title} className={className}>
      {message}
      {onRetry && (
        <>
          {" "}
          <Button variant="ghost" size="sm" onClick={onRetry} className="h-auto px-1.5 py-0.5 align-baseline">
            {retryLabel}
          </Button>
        </>
      )}
    </Notice>
  );
}
