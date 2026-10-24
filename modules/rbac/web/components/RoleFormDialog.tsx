import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Alert, AlertDescription } from "@/shared/components/primitives/alert";
import { Button } from "@/shared/components/primitives/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/components/primitives/dialog";
import { Input } from "@/shared/components/primitives/input";
import { Spinner } from "@/shared/components/primitives/spinner";
import { Textarea } from "@/shared/components/primitives/textarea";
import { FormField } from "@/shared/components/ui/form-field";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { createRole, updateRole } from "@/modules/rbac/services/rbac.service";
import type { Role } from "@/modules/rbac/types/rbac";

interface RoleFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The role to rename; absent to create one (with no permissions yet). */
  role?: Role;
  onSaved: (role: Role) => void;
}

/** Name and description of a role. Permissions are edited on the role's page. */
export function RoleFormDialog({ open, onOpenChange, role, onSaved }: RoleFormDialogProps) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setName(role?.name ?? "");
    setDescription(role?.description ?? "");
    setErrors({});
    setFormError(null);
  }, [open, role]);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!name.trim()) {
      setErrors({ name: "Name is required" });
      return;
    }
    setSaving(true);
    setErrors({});
    setFormError(null);
    try {
      const values = { name, description, permissions: role?.permissions ?? [] };
      onSaved(role ? await updateRole(role.id, values) : await createRole(values));
      onOpenChange(false);
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (Object.keys(fields).length) setErrors(fields);
      else setFormError(apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !saving && onOpenChange(next)}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{role ? "Edit role" : "New role"}</DialogTitle>
          <DialogDescription>
            {role ? "Rename the role or change its description." : "Choose its permissions next, on the role's page."}
          </DialogDescription>
        </DialogHeader>
        <form id="role-form" onSubmit={submit} noValidate className="grid gap-4">
          {formError && (
            <Alert variant="destructive">
              <AlertDescription>{formError}</AlertDescription>
            </Alert>
          )}
          <FormField label="Name" error={errors.name}>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Warehouse staff" />
          </FormField>
          <FormField label="Description" error={errors.description} hint="Optional — who this role is for.">
            <Textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={3} />
          </FormField>
        </form>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            Cancel
          </Button>
          <Button type="submit" form="role-form" disabled={saving}>
            {saving && <Spinner />}
            {role ? "Save" : "Create role"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
