import type { ReactNode } from "react";
import { MoreHorizontal } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/shared/components/primitives/dropdown-menu";

export interface ActionMenuItem {
  key: string;
  label: string;
  icon?: ReactNode;
  onSelect: () => void;
  /** Destructive: red, separated, last. */
  destructive?: boolean;
  disabled?: boolean;
}

export interface ActionMenuProps {
  items: ActionMenuItem[];
  label?: string;
  align?: "start" | "end";
  /** "outline" in a page header, "ghost" in a table row. */
  variant?: "outline" | "ghost";
  size?: "icon" | "icon-sm";
}

/** The "⋯" menu for secondary actions. The main action stays a visible button. */
export function ActionMenu({ items, label = "Aksi lainnya", align = "end", variant = "outline", size = "icon" }: ActionMenuProps) {
  if (items.length === 0) return null;
  const normal = items.filter((i) => !i.destructive);
  const danger = items.filter((i) => i.destructive);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant={variant} size={size} aria-label={label} className={variant === "ghost" ? "text-muted-foreground hover:text-foreground" : undefined}>
          <MoreHorizontal aria-hidden />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align={align} className="min-w-48">
        {normal.map((i) => (
          <DropdownMenuItem key={i.key} disabled={i.disabled} onSelect={i.onSelect}>
            {i.icon}
            {i.label}
          </DropdownMenuItem>
        ))}
        {normal.length > 0 && danger.length > 0 && <DropdownMenuSeparator />}
        {danger.map((i) => (
          <DropdownMenuItem key={i.key} variant="destructive" disabled={i.disabled} onSelect={i.onSelect}>
            {i.icon}
            {i.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
