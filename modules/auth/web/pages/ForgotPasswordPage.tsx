import { useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";
import { FormField } from "@/shared/components/ui/form-field";
import { Notice } from "@/shared/components/ui/notice";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { AuthLayout } from "@/modules/auth/components/AuthLayout";
import { AUTH_PATHS } from "@/modules/auth/paths";
import { requestReset } from "@/modules/auth/services/auth.service";

/** "Forgot password": the answer is the same whether or not the email has an account. */
export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string>();
  const [sent, setSent] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!email.trim()) {
      setFieldError("Email wajib diisi");
      return;
    }
    setBusy(true);
    setError(null);
    setFieldError(undefined);
    try {
      setSent(await requestReset(email));
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (fields.email) setFieldError(fields.email);
      else setError(apiErrorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthLayout title="Lupa kata sandi" description="Masukkan email akun Anda. Kami kirimkan tautan untuk membuat kata sandi baru.">
      {sent ? (
        <div className="grid gap-4">
          <Notice tone="success" title="Periksa email Anda">{sent}</Notice>
          <p className="text-sm text-muted-foreground">Tautan berlaku 30 menit. Tidak menerima email? Periksa folder spam, atau coba lagi beberapa menit kemudian.</p>
        </div>
      ) : (
        <form onSubmit={submit} noValidate className="grid gap-4">
          {error && <Notice tone="danger" title="Gagal mengirim">{error}</Notice>}
          <FormField label="Email" error={fieldError}>
            <Input type="email" autoComplete="email" autoFocus value={email} onChange={(e) => setEmail(e.target.value)} />
          </FormField>
          <Button type="submit" className="mt-2 w-full" loading={busy}>
            Kirim tautan
          </Button>
        </form>
      )}
      <Button variant="ghost" size="sm" asChild className="mt-6 -ml-2">
        <Link to={AUTH_PATHS.login}>
          <ArrowLeft />
          Kembali ke halaman masuk
        </Link>
      </Button>
    </AuthLayout>
  );
}
