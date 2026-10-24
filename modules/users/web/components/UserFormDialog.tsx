import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/components/primitives/dialog";
import { Input } from "@/shared/components/primitives/input";
import { Label } from "@/shared/components/primitives/label";
import { Spinner } from "@/shared/components/primitives/spinner";
import { Alert, AlertDescription } from "@/shared/components/primitives/alert";
import { FormField } from "@/shared/components/ui/form-field";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { createUserSchema, updateUserSchema } from "@/modules/users/schemas/user.schema";
import { createUser, updateUser } from "@/modules/users/services/users.service";
import type { User } from "@/modules/users/types/user";

interface UserFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The user to edit; absent to create one. */
  user?: User;
  onSaved: (user: User) => void;
}

type Errors = Partial<Record<"name" | "email" | "password", string>>;

/** Create or edit a user — four fields, so a dialog rather than a page. */
export function UserFormDialog({ open, onOpenChange, user, onSaved }: UserFormDialogProps) {
  const editing = Boolean(user);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isActive, setIsActive] = useState(true);
  const [errors, setErrors] = useState<Errors>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setName(user?.name ?? "");
    setEmail(user?.email ?? "");
    setPassword("");
    setIsActive(user?.is_active ?? true);
    setErrors({});
    setFormError(null);
  }, [open, user]);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const values = { name, email, password, is_active: isActive };
    const parsed = user ? updateUserSchema.safeParse(values) : createUserSchema.safeParse(values);
    if (!parsed.success) {
      const next: Errors = {};
      for (const issue of parsed.error.issues) next[issue.path[0] as keyof Errors] ??= issue.message;
      setErrors(next);
      return;
    }
    setErrors({});
    setFormError(null);
    setSaving(true);
    try {
      const saved = user ? await updateUser(user.id, parsed.data) : await createUser(createUserSchema.parse(values));
      onSaved(saved);
      onOpenChange(false);
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (Object.keys(fields).length > 0) setErrors(fields);
      else setFormError(apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !saving && onOpenChange(next)}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editing ? "Edit user" : "Add user"}</DialogTitle>
          <DialogDescription>
            {editing ? "Leave the password empty to keep the current one." : "They sign in with this email and password."}
          </DialogDescription>
        </DialogHeader>
        <form id="user-form" onSubmit={submit} noValidate className="grid gap-4">
          {formError && (
            <Alert variant="destructive">
              <AlertDescription>{formError}</AlertDescription>
            </Alert>
          )}
          <FormField label="Name" error={errors.name}>
            <Input value={name} onChange={(e) => setName(e.target.value)} autoComplete="off" />
          </FormField>
          <FormField label="Email" error={errors.email}>
            <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="off" />
          </FormField>
          <FormField
            label={editing ? "New password" : "Password"}
            error={errors.password}
            hint="At least 8 characters."
          >
            <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" />
          </FormField>
          <div className="flex items-center gap-2">
            <Checkbox id="user-active" checked={isActive} onCheckedChange={(v) => setIsActive(v === true)} />
            <Label htmlFor="user-active">Active — can sign in</Label>
          </div>
        </form>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            Cancel
          </Button>
          <Button type="submit" form="user-form" disabled={saving}>
            {saving && <Spinner />}
            {editing ? "Save" : "Add user"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
