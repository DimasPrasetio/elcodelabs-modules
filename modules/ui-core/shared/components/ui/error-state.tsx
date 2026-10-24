import { TriangleAlert } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";

export interface ErrorStateProps {
  /** The API's own message — never a generic "Something went wrong" when a real one exists. */
  message: string;
  onRetry?: () => void;
}

/** A failed load, with the reason and a way to try again. */
export function ErrorState({ message, onRetry }: ErrorStateProps) {
  return (
    <div role="alert" className="flex flex-col items-center justify-center gap-3 px-6 py-12 text-center">
      <div className="flex size-10 items-center justify-center rounded-full bg-destructive-soft text-destructive">
        <TriangleAlert className="size-5" />
      </div>
      <p className="max-w-sm text-sm">{message}</p>
      {onRetry && (
        <Button variant="outline" size="sm" onClick={onRetry}>
          Try again
        </Button>
      )}
    </div>
  );
}
