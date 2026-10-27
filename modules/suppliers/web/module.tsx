import { lazy } from "react";
import { Truck } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";

const SuppliersPage = lazy(() => import("@/modules/suppliers/pages/SuppliersPage"));
const SupplierPage = lazy(() => import("@/modules/suppliers/pages/SupplierPage"));

export const suppliersModule: FrontendModule = {
  id: "suppliers",
  protected: [
    { path: "/suppliers", element: <SuppliersPage /> },
    { path: "/suppliers/:id", element: <SupplierPage /> },
  ],
  // Purchase documents take 200–280 of "Pembelian"; master data closes the group.
  nav: [{ label: "Pemasok", to: "/suppliers", icon: Truck, group: "Pembelian", order: 290, permission: "suppliers.read" }],
};
