import { useNavigate } from "react-router-dom";
import { ChevronsUpDown, LogOut } from "lucide-react";
import { Avatar, AvatarFallback } from "@/shared/components/primitives/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/shared/components/primitives/dropdown-menu";
import { useSessionStore } from "@/modules/auth/stores/session.store";
import { AUTH_PATHS } from "@/modules/auth/paths";

function initials(name: string) {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]!.toUpperCase())
    .join("");
}

/** The signed-in user and "Sign out" — contributed to the layout's "sidebar-footer" slot. */
export function UserMenu() {
  const { user, signOut } = useSessionStore();
  const navigate = useNavigate();
  if (!user) return null;

  async function handleSignOut() {
    await signOut();
    navigate(AUTH_PATHS.login, { replace: true });
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="flex w-full items-center gap-2 rounded-md p-2 text-left text-sm text-sidebar-foreground outline-hidden hover:bg-sidebar-accent focus-visible:ring-2 focus-visible:ring-sidebar-ring data-[state=open]:bg-sidebar-accent">
        <Avatar className="size-8 rounded-md">
          <AvatarFallback className="rounded-md bg-sidebar-accent text-xs font-medium text-sidebar-strong">
            {initials(user.name)}
          </AvatarFallback>
        </Avatar>
        <span className="grid flex-1 leading-tight group-data-[collapsible=icon]:hidden">
          <span className="truncate font-medium text-sidebar-strong">{user.name}</span>
          <span className="truncate text-xs text-sidebar-muted">{user.email}</span>
        </span>
        <ChevronsUpDown className="size-4 shrink-0 text-sidebar-muted group-data-[collapsible=icon]:hidden" />
      </DropdownMenuTrigger>
      <DropdownMenuContent side="right" align="end" className="min-w-56">
        <DropdownMenuLabel className="font-normal">
          <div className="font-medium">{user.name}</div>
          <div className="text-xs text-muted-foreground">{user.email}</div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={handleSignOut}>
          <LogOut />
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
