import { Fragment } from "react";
import { Building2, Check, ChevronsUpDown, Star } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/shared/components/primitives/dropdown-menu";
import { setScope } from "@/shared/scope";
import { notify } from "@/shared/lib/notify";
import { useOrganization } from "@/modules/organization/context/OrganizationProvider";
import { setMyDefault } from "@/modules/organization/services/organization.service";
import { unitKey } from "@/modules/organization/types/organization";
import type { Unit } from "@/modules/organization/types/organization";

function byCompany(units: Unit[]): Unit[][] {
  const groups = new Map<number, Unit[]>();
  for (const u of units) groups.set(u.company_id, [...(groups.get(u.company_id) ?? []), u]);
  return [...groups.values()];
}

/**
 * Where this tab works — "PT Maju › Bandung" — in the header (slot "shell:scope"). Choosing a
 * place switches this tab only (other tabs keep theirs); "Jadikan default" makes it where new
 * tabs and sign-ins start. Hidden when the user can work in one place only.
 */
export function ScopePicker() {
  const { me, reload } = useOrganization();
  if (!me || me.units.length <= 1) return null;
  const current = me.units.find((u) => u.company_id === me.current.company_id && u.branch_id === me.current.branch_id);
  const isDefault = me.default.company_id === me.current.company_id && me.default.branch_id === me.current.branch_id;

  async function makeDefault() {
    if (!current) return;
    try {
      await setMyDefault(current.company_id, current.branch_id);
      notify.success("Tempat kerja default disimpan");
      reload();
    } catch (err) {
      notify.error(err);
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="flex max-w-72 items-center gap-2 rounded-md border border-border px-2.5 py-1 text-left text-sm outline-hidden hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring data-[state=open]:bg-accent">
        <Building2 className="size-4 shrink-0 text-muted-foreground" />
        <span className="truncate">
          <span className="font-medium">{current?.company_name ?? "Perusahaan"}</span>
          <span className="text-muted-foreground">{current?.branch_id ? ` › ${current.branch_name}` : " · seluruh perusahaan"}</span>
        </span>
        <ChevronsUpDown className="size-3.5 shrink-0 text-muted-foreground" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="max-h-96 min-w-64 overflow-y-auto">
        {byCompany(me.units).map((units, i) => (
          <Fragment key={units[0]!.company_id}>
            {i > 0 && <DropdownMenuSeparator />}
            <DropdownMenuLabel className="text-xs text-muted-foreground">{units[0]!.company_name}</DropdownMenuLabel>
            {units.map((u) => {
              const active = u.company_id === me.current.company_id && u.branch_id === me.current.branch_id;
              return (
                <DropdownMenuItem key={unitKey(u.company_id, u.branch_id)} onSelect={() => !active && setScope(unitKey(u.company_id, u.branch_id))}>
                  <Check className={active ? "" : "invisible"} />
                  {u.branch_id ? u.branch_name : "Seluruh perusahaan"}
                </DropdownMenuItem>
              );
            })}
          </Fragment>
        ))}
        {current?.placed && !isDefault && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={makeDefault}>
              <Star />
              Jadikan default
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
