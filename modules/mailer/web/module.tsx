import { lazy } from "react";
import { Mail } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";

const OutboxPage = lazy(() => import("@/modules/mailer/pages/OutboxPage"));

export const MAILER_PATHS = {
  outbox: "/mailer/outbox",
} as const;

export const mailerModule: FrontendModule = {
  id: "mailer",
  protected: [{ path: MAILER_PATHS.outbox, element: <OutboxPage /> }],
  nav: [{ label: "Kotak keluar", to: MAILER_PATHS.outbox, icon: Mail, group: "Pengaturan", order: 940, permission: "mailer.read" }],
};
