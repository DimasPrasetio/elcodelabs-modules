import { lazy } from "react";
import { FolderOpen } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";

const FilesPage = lazy(() => import("@/modules/files/pages/FilesPage"));

export const FILES_PATHS = {
  list: "/files",
} as const;

export const filesModule: FrontendModule = {
  id: "files",
  protected: [{ path: FILES_PATHS.list, element: <FilesPage /> }],
  nav: [{ label: "Berkas", to: FILES_PATHS.list, icon: FolderOpen, group: "Pengaturan", order: 960, permission: "files.read" }],
};
