import { Navigate, createBrowserRouter } from "react-router-dom";
import type { RouteObject } from "react-router-dom";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { frontendModules } from "@/app/modules";
import { BareLayout } from "@/app/layouts/BareLayout";
import { NotFoundPage, RouteErrorPage } from "@/shared/components/feedback/RouteError";

const publicRoutes = frontendModules.flatMap((m) => m.public ?? []);
// An unknown address inside the app shows "not found" in the app's own frame.
let protectedRoutes: RouteObject[] = [...frontendModules.flatMap((m) => m.protected ?? []), { path: "*", element: <NotFoundPage /> }];

// A layout module frames the protected pages (BareLayout until one is installed); a guard
// module decides who reaches them.
const layout = frontendModules.find((m) => m.layout)?.layout ?? <BareLayout />;
protectedRoutes = [{ element: layout, children: protectedRoutes }];
const guard = frontendModules.find((m) => m.guard)?.guard;
if (guard) protectedRoutes = [{ element: guard, children: protectedRoutes }];

// One root catches whatever a page throws (or a page chunk an older tab can no longer load)
// with the app's own error page instead of the router's built-in one.
export const router = createBrowserRouter([
  {
    errorElement: <RouteErrorPage />,
    children: [
      { path: ROUTE_PATHS.home, element: <Navigate to={ROUTE_PATHS.start} replace /> },
      ...publicRoutes,
      ...protectedRoutes,
    ],
  },
]);
