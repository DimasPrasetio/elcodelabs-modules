import { useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { FormField } from "@/shared/components/ui/form-field";
import { Notice } from "@/shared/components/ui/notice";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { PasswordInput } from "@/shared/components/ui/password-input";
import { SectionCard } from "@/shared/components/ui/section-card";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { changePassword } from "@/modules/auth/services/auth.service";

/** The signed-in user changes their own password; other devices are signed out. */
export default function ChangePasswordPage() {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const local: Record<string, string> = {};
    if (!current) local.current_password = "Wajib diisi";
    if (next.length < 8) local.new_password = "Minimal 8 karakter";
    if (confirm !== next) local.confirm = "Tidak sama dengan kata sandi baru";
    setErrors(local);
    if (Object.keys(local).length) return;
    setBusy(true);
    setError(null);
    try {
      notify.success(await changePassword(current, next));
      setCurrent("");
      setNext("");
      setConfirm("");
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (Object.keys(fields).length) setErrors(fields);
      else setError(apiErrorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Page>
      <PageHeader title="Kata sandi" description="Ganti kata sandi akun Anda. Perangkat lain yang masih masuk akan dikeluarkan." />
      <SectionCard title="Ganti kata sandi" className="max-w-xl">
        <form onSubmit={submit} noValidate className="grid gap-4">
          {error && <Notice tone="danger" title="Gagal menyimpan">{error}</Notice>}
          <FormField label="Kata sandi saat ini" error={errors.current_password}>
            <PasswordInput autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} />
          </FormField>
          <FormField label="Kata sandi baru" error={errors.new_password} hint="Minimal 8 karakter, berbeda dari yang sekarang.">
            <PasswordInput autoComplete="new-password" value={next} onChange={(e) => setNext(e.target.value)} />
          </FormField>
          <FormField label="Ulangi kata sandi baru" error={errors.confirm}>
            <PasswordInput autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} />
          </FormField>
          <div>
            <Button type="submit" loading={busy}>
              Simpan kata sandi
            </Button>
          </div>
        </form>
      </SectionCard>
    </Page>
  );
}
