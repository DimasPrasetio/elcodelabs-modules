import { lazy } from "react";
import { Contact } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";

const CustomersPage = lazy(() => import("@/modules/customers/pages/CustomersPage"));
const CustomerPage = lazy(() => import("@/modules/customers/pages/CustomerPage"));

export const customersModule: FrontendModule = {
  id: "customers",
  protected: [
    { path: "/customers", element: <CustomersPage /> },
    { path: "/customers/:id", element: <CustomerPage /> },
  ],
  // Sales documents take 100–180 of "Penjualan"; master data closes the group.
  nav: [{ label: "Pelanggan", to: "/customers", icon: Contact, group: "Penjualan", order: 190, permission: "customers.read" }],
};
