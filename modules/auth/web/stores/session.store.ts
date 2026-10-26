import axios from "axios";
import { create } from "zustand";
import { emitAppEvent, onAppEvent } from "@/shared/app-events";
import { fetchMe, login, logout } from "@/modules/auth/services/auth.service";
import type { SessionUser } from "@/modules/auth/services/auth.service";
import type { LoginFormValues } from "@/modules/auth/schemas/login.schema";

type Status = "unknown" | "loading" | "signed-in" | "signed-out";

interface SessionState {
  status: Status;
  user: SessionUser | null;
  /** Asks the API who is signed in (once per page load, or after a 401). */
  load: () => Promise<void>;
  signIn: (values: LoginFormValues) => Promise<void>;
  signOut: () => Promise<void>;
  /** Forget the session locally — the API already said it is gone (401). */
  expire: () => void;
}

export const useSessionStore = create<SessionState>((set, get) => ({
  status: "unknown",
  user: null,
  load: async () => {
    if (get().status === "loading") return;
    set({ status: "loading" });
    try {
      set({ status: "signed-in", user: await fetchMe() });
    } catch (err) {
      if (axios.isAxiosError(err) && err.response?.status === 401) set({ status: "signed-out", user: null });
      else set({ status: "unknown", user: null });
      if (!(axios.isAxiosError(err) && err.response?.status === 401)) throw err;
    }
  },
  // Every change of who is signed in is announced, so other modules (e.g. permissions) reload.
  signIn: async (values) => {
    const user = await login(values);
    set({ status: "signed-in", user });
    emitAppEvent("session:changed");
  },
  signOut: async () => {
    try {
      await logout();
    } finally {
      set({ status: "signed-out", user: null });
      emitAppEvent("session:changed");
    }
  },
  expire: () => {
    if (get().status === "signed-out") return;
    set({ status: "signed-out", user: null });
    emitAppEvent("session:changed");
  },
}));

// The user's own profile changed elsewhere (their name): read it again, quietly.
onAppEvent("profile:changed", () => {
  const { status } = useSessionStore.getState();
  if (status !== "signed-in") return;
  fetchMe()
    .then((user) => useSessionStore.setState({ user }))
    .catch(() => undefined);
});
