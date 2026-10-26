import { useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/shared/components/primitives/card";
import { Input } from "@/shared/components/primitives/input";
import { Label } from "@/shared/components/primitives/label";
import { loginSchema } from "@/modules/auth/schemas/login.schema";
import type { LoginFormValues } from "@/modules/auth/schemas/login.schema";

type FieldErrors = Partial<Record<keyof LoginFormValues, string>>;

export default function LoginPage() {
  const [errors, setErrors] = useState<FieldErrors>({});

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const formData = new FormData(e.currentTarget);
    const result = loginSchema.safeParse({
      email: formData.get("email"),
      password: formData.get("password"),
    });
    if (!result.success) {
      const next: FieldErrors = {};
      for (const issue of result.error.issues) {
        const field = issue.path[0] as keyof LoginFormValues;
        next[field] ??= issue.message;
      }
      setErrors(next);
      return;
    }
    setErrors({});
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="font-display text-xl">Masuk</CardTitle>
          <CardDescription>Gunakan akun Anda untuk melanjutkan.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} noValidate className="grid gap-4">
            <div className="grid gap-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                name="email"
                type="email"
                autoComplete="email"
                aria-invalid={errors.email ? true : undefined}
                aria-describedby={errors.email ? "email-error" : undefined}
              />
              {errors.email && <p id="email-error" className="text-sm text-destructive">{errors.email}</p>}
            </div>
            <div className="grid gap-2">
              <Label htmlFor="password">Kata sandi</Label>
              <Input
                id="password"
                name="password"
                type="password"
                autoComplete="current-password"
                aria-invalid={errors.password ? true : undefined}
                aria-describedby={errors.password ? "password-error" : undefined}
              />
              {errors.password && <p id="password-error" className="text-sm text-destructive">{errors.password}</p>}
            </div>
            <Button type="submit" className="w-full">Masuk</Button>
          </form>
        </CardContent>
      </Card>
    </main>
  );
}
