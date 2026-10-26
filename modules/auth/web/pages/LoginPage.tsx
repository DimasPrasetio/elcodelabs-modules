import { useState } from "react";
import type { FormEvent } from "react";
import { Link, Navigate, useNavigate, useSearchParams } from "react-router-dom";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";
import { Notice } from "@/shared/components/ui/notice";
import { FormField } from "@/shared/components/ui/form-field";
import { PasswordInput } from "@/shared/components/ui/password-input";
import { apiErrorMessage } from "@/shared/services/api-error";
import { loginSchema } from "@/modules/auth/schemas/login.schema";
import type { LoginFormValues } from "@/modules/auth/schemas/login.schema";
import { useSessionStore } from "@/modules/auth/stores/session.store";
import { AuthLayout } from "@/modules/auth/components/AuthLayout";
import { useAuthOptions } from "@/modules/auth/hooks/use-auth-options";
import { AUTH_PATHS } from "@/modules/auth/paths";

type FieldErrors = Partial<Record<keyof LoginFormValues, string>>;

/** Only same-app paths are followed after sign-in — never another origin. */
function safeNext(next: string | null) {
  return next && next.startsWith("/") && !next.startsWith("//") ? next : "/";
}

export default function LoginPage() {
  const { status, signIn } = useSessionStore();
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [errors, setErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const next = safeNext(params.get("next"));
  const options = useAuthOptions();

  if (status === "signed-in") return <Navigate to={next} replace />;

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const result = loginSchema.safeParse({ email, password });
    if (!result.success) {
      const fields: FieldErrors = {};
      for (const issue of result.error.issues) fields[issue.path[0] as keyof LoginFormValues] ??= issue.message;
      setErrors(fields);
      return;
    }
    setErrors({});
    setFormError(null);
    setBusy(true);
    try {
      await signIn(result.data);
      navigate(next, { replace: true });
    } catch (err) {
      setFormError(apiErrorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthLayout title="Masuk" description="Gunakan email kerja dan kata sandi Anda.">
      <form onSubmit={handleSubmit} noValidate className="grid gap-4">
        {formError && <Notice tone="danger" title="Gagal masuk">{formError}</Notice>}
        <FormField label="Email" error={errors.email}>
          <Input type="email" autoComplete="username" autoFocus value={email} onChange={(e) => setEmail(e.target.value)} />
        </FormField>
        <FormField
          label="Kata sandi"
          error={errors.password}
          labelAside={
            options.password_reset && (
              <Link to={AUTH_PATHS.forgot} className="text-xs font-medium text-primary hover:underline">
                Lupa kata sandi?
              </Link>
            )
          }
        >
          <PasswordInput autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} />
        </FormField>
        <Button type="submit" className="mt-2 w-full" loading={busy}>
          Masuk
        </Button>
      </form>
    </AuthLayout>
  );
}
