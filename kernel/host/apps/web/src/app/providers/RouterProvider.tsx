import { Suspense } from "react";
import { RouterProvider as ReactRouterProvider } from "react-router-dom";
import { router } from "@/app/routes";
import { LoadingState } from "@/shared/components/feedback/LoadingState";

export function RouterProvider() {
  return (
    <Suspense fallback={<LoadingState />}>
      <ReactRouterProvider router={router} />
    </Suspense>
  );
}
