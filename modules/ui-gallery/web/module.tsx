import { lazy } from "react";
import { Palette } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";

const GalleryPage = lazy(() => import("@/modules/ui-gallery/pages/GalleryPage"));

export const uiGalleryModule: FrontendModule = {
  id: "ui-gallery",
  protected: [{ path: "/ui-gallery", element: <GalleryPage /> }],
  nav: [{ label: "Galeri UI", to: "/ui-gallery", icon: Palette, group: "Pengembang", order: 9990 }],
};
