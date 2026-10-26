import { Outlet } from "react-router-dom";

/** The frame around protected pages until a layout module (an app shell) is installed. */
export function BareLayout() {
  return (
    <main className="min-h-screen bg-background p-4 md:p-8">
      <Outlet />
    </main>
  );
}
