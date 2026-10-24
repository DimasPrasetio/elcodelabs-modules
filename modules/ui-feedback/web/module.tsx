import type { FrontendModule } from "@/shared/types/frontend-module";
import { FeedbackProvider } from "@/modules/ui-feedback/FeedbackProvider";

export const uiFeedbackModule: FrontendModule = {
  id: "ui-feedback",
  provider: FeedbackProvider,
};
