import type { ReactNode } from "react";
import { Toaster } from "@/shared/components/primitives/sonner";
import { ErrorBoundary } from "@/shared/components/ui/error-boundary";
import { GlobalLoader } from "@/shared/components/ui/global-loader";

/** Mounted once around the app (the registry's provider): crash card, loading bar, toasts. */
export function FeedbackProvider({ children }: { children: ReactNode }) {
  return (
    <ErrorBoundary fullScreen>
      <GlobalLoader />
      {children}
      <Toaster position="bottom-right" closeButton visibleToasts={5} />
    </ErrorBoundary>
  );
}
