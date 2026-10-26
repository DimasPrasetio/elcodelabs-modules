import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Link, useLocation } from "react-router-dom";
import { Button } from "@/shared/components/primitives/button";
import { FormField } from "@/shared/components/ui/form-field";
import { Notice } from "@/shared/components/ui/notice";
import { PasswordInput } from "@/shared/components/ui/password-input";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { AuthLayout } from "@/modules/auth/components/AuthLayout";
import { AUTH_PATHS } from "@/modules/auth/paths";
import { resetPassword } from "@/modules/auth/services/auth.service";

/**
 * The page a reset email links to: choose a new password. The token is in the address fragment
 * (#token=…), which browsers never send to a server; it is taken out of the address bar once read.
 */
export default function ResetPasswordPage() {
  const { hash, pathname, search } = useLocation();
  const [token] = useState(() => new URLSearchParams(hash.slice(1)).get("token") ?? "");
  useEffect(() => {
    if (hash) window.history.replaceState(window.history.state, "", pathname + search);
  }, [hash, pathname, search]);
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const local: Record<string, string> = {};
    if (password.length < 8) local.new_password = "Minimal 8 karakter";
    if (confirm !== password) local.confirm = "Tidak sama dengan kata sandi baru";
    setErrors(local);
    if (Object.keys(local).length) return;
    setBusy(true);
    setError(null);
    try {
      setDone(await resetPassword(token, password));
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (Object.keys(fields).length) setErrors(fields);
      else setError(apiErrorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  if (!token) {
    return (
      <AuthLayout title="Tautan tidak lengkap">
        <Notice tone="danger" title="Tautan tidak valid">Buka tautan dari email persis seperti yang dikirim, atau minta tautan baru.</Notice>
        <Button asChild className="mt-6 w-full">
          <Link to={AUTH_PATHS.forgot}>Minta tautan baru</Link>
        </Button>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout title="Buat kata sandi baru" description="Setelah disimpan, semua perangkat yang masih masuk ke akun ini akan dikeluarkan.">
      {done ? (
        <div className="grid gap-4">
          <Notice tone="success" title="Kata sandi diperbarui">{done}</Notice>
          <Button asChild className="w-full">
            <Link to={AUTH_PATHS.login}>Masuk</Link>
          </Button>
        </div>
      ) : (
        <form onSubmit={submit} noValidate className="grid gap-4">
          {error && (
            <Notice tone="danger" title="Gagal menyimpan">
              {error}{" "}
              <Link to={AUTH_PATHS.forgot} className="font-medium underline">
                Minta tautan baru
              </Link>
            </Notice>
          )}
          <FormField label="Kata sandi baru" error={errors.new_password} hint="Minimal 8 karakter.">
            <PasswordInput autoComplete="new-password" autoFocus value={password} onChange={(e) => setPassword(e.target.value)} />
          </FormField>
          <FormField label="Ulangi kata sandi baru" error={errors.confirm}>
            <PasswordInput autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} />
          </FormField>
          <Button type="submit" className="mt-2 w-full" loading={busy}>
            Simpan kata sandi
          </Button>
        </form>
      )}
    </AuthLayout>
  );
}
