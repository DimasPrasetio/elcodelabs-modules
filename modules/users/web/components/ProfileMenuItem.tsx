import { Link } from "react-router-dom";
import { UserRound } from "lucide-react";
import { DropdownMenuItem } from "@/shared/components/primitives/dropdown-menu";

/** "Profil saya" in the user menu — contributed to the "user-menu:items" slot. */
export function ProfileMenuItem() {
  return (
    <DropdownMenuItem asChild>
      <Link to="/account">
        <UserRound />
        Profil saya
      </Link>
    </DropdownMenuItem>
  );
}
