import { toast } from "sonner";
import { apiErrorMessage } from "@/shared/services/api-error";

/**
 * Short confirmations and failures as toasts. An error shows the API's own message.
 * Use a toast for something that happened; use a Notice for something the user must read.
 */
export const notify = {
  success: (message: string) => toast.success(message),
  info: (message: string) => toast.info(message),
  warning: (message: string) => toast.warning(message),
  error: (err: unknown, fallback?: string) => toast.error(typeof err === "string" ? err : apiErrorMessage(err, fallback)),
};
