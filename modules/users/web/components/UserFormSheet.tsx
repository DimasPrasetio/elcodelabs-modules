import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";
import { Switch } from "@/shared/components/primitives/switch";
import { FormSheet } from "@/shared/components/ui/form-sheet";
import { Notice } from "@/shared/components/ui/notice";
import { FormField } from "@/shared/components/ui/form-field";
import { PasswordInput } from "@/shared/components/ui/password-input";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { notify } from "@/shared/lib/notify";
import { createUserSchema, updateUserSchema } from "@/modules/users/schemas/user.schema";
import { createUser, updateUser } from "@/modules/users/services/users.service";
import type { User } from "@/modules/users/types/user";

interface UserFormSheetProps {
  open: boolean;
  onClose: () => void;
  /** The user to edit; absent to add one. */
  user?: User;
  onSaved: (user: User) => void;
}

type Errors = Partial<Record<"name" | "email" | "password", string>>;

/** Add or edit a user — master data, so a side sheet. */
export function UserFormSheet({ open, onClose, user, onSaved }: UserFormSheetProps) {
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
      notify.success(user ? "Pengguna diperbarui" : "Pengguna ditambahkan");
      onSaved(saved);
      onClose();
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (Object.keys(fields).length > 0) setErrors(fields);
      else setFormError(apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <FormSheet
      open={open}
      onClose={() => !saving && onClose()}
      title={editing ? "Ubah pengguna" : "Tambah pengguna"}
      description={editing ? "Kosongkan kata sandi untuk tetap memakai yang lama." : "Pengguna masuk dengan email dan kata sandi ini."}
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button type="submit" form="user-form" loading={saving}>
            {editing ? "Simpan" : "Tambah pengguna"}
          </Button>
        </>
      }
    >
      <form id="user-form" onSubmit={submit} noValidate className="grid gap-4">
        {formError && <Notice tone="danger" title="Gagal menyimpan">{formError}</Notice>}
        <FormField label="Nama" required error={errors.name}>
          <Input value={name} onChange={(e) => setName(e.target.value)} autoComplete="off" />
        </FormField>
        <FormField label="Email" required error={errors.email}>
          <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="off" />
        </FormField>
        <FormField label={editing ? "Kata sandi baru" : "Kata sandi"} required={!editing} error={errors.password} hint="Minimal 8 karakter.">
          <PasswordInput value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" />
        </FormField>
        <label className="flex items-center justify-between gap-4 rounded-lg border border-border px-4 py-3">
          <span>
            <span className="block text-sm font-medium">Aktif</span>
            <span className="block text-xs text-muted-foreground">Pengguna nonaktif tidak bisa masuk.</span>
          </span>
          <Switch checked={isActive} onCheckedChange={setIsActive} />
        </label>
      </form>
    </FormSheet>
  );
}
