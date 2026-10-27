import { lazy } from "react";
import { Hash } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";

const NumberingPage = lazy(() => import("@/modules/numbering/pages/NumberingPage"));

export const NUMBERING_PATHS = {
  root: "/numbering",
} as const;

export const numberingModule: FrontendModule = {
  id: "numbering",
  protected: [{ path: NUMBERING_PATHS.root, element: <NumberingPage /> }],
  nav: [{ label: "Penomoran", to: NUMBERING_PATHS.root, icon: Hash, group: "Pengaturan", order: 810, permission: "numbering.read" }],
};
