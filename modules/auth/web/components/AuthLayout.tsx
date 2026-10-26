import type { ReactNode } from "react";

/** The signed-out pages' frame: brand panel on the left (wide screens), the form on the right. */
export function AuthLayout({ title, description, children }: { title: string; description?: ReactNode; children: ReactNode }) {
  const appName = document.title || "Aplikasi";
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
          <h1 className="font-display text-2xl font-semibold tracking-tight">{title}</h1>
          {description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
          <div className="mt-8">{children}</div>
        </div>
      </section>
    </main>
  );
}
