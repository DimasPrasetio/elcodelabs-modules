import { useState } from "react";
import type { FormEvent } from "react";
import { Navigate, useNavigate, useSearchParams } from "react-router-dom";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";
import { Notice } from "@/shared/components/ui/notice";
import { FormField } from "@/shared/components/ui/form-field";
import { PasswordInput } from "@/shared/components/ui/password-input";
import { apiErrorMessage } from "@/shared/services/api-error";
import { loginSchema } from "@/modules/auth/schemas/login.schema";
import type { LoginFormValues } from "@/modules/auth/schemas/login.schema";
import { useSessionStore } from "@/modules/auth/stores/session.store";

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
  const appName = document.title || "Aplikasi";

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
    <main className="grid min-h-screen bg-background lg:grid-cols-[5fr_7fr]">
      <aside className="hidden flex-col justify-between bg-sidebar p-10 text-sidebar-foreground lg:flex">
        <div className="flex items-center gap-2">
          <div className="flex size-8 items-center justify-center rounded-md bg-sidebar-primary font-display font-semibold text-sidebar-primary-foreground">
            {appName.charAt(0).toUpperCase()}
          </div>
          <span className="font-display font-semibold text-sidebar-strong">{appName}</span>
        </div>
        <p className="max-w-sm font-display text-2xl leading-snug font-semibold text-sidebar-strong">Semua yang dijalankan tim Anda, di satu tempat.</p>
        <span className="text-xs text-sidebar-muted">
          © {new Date().getFullYear()} {appName}
        </span>
      </aside>
      <section className="flex items-center justify-center p-6">
        <div className="w-full max-w-sm">
          <h1 className="font-display text-2xl font-semibold tracking-tight">Masuk</h1>
          <p className="mt-1 text-sm text-muted-foreground">Gunakan email kerja dan kata sandi Anda.</p>
          <form onSubmit={handleSubmit} noValidate className="mt-8 grid gap-4">
            {formError && <Notice tone="danger" title="Gagal masuk">{formError}</Notice>}
            <FormField label="Email" error={errors.email}>
              <Input type="email" autoComplete="username" autoFocus value={email} onChange={(e) => setEmail(e.target.value)} />
            </FormField>
            <FormField label="Kata sandi" error={errors.password}>
              <PasswordInput autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} />
            </FormField>
            <Button type="submit" className="mt-2 w-full" loading={busy}>
              Masuk
            </Button>
          </form>
        </div>
      </section>
    </main>
  );
}
