import { TriangleAlert } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";

/**
 * Shown instead of a white screen when a page fails to render. stale = a new version was
 * deployed and this page's code is gone: reloading fixes it.
 */
export function CrashCard({ errorId, stale = false, onReload, fullScreen = false }: { errorId?: string; stale?: boolean; onReload?: () => void; fullScreen?: boolean }) {
  const reload = onReload ?? (() => window.location.reload());
  const card = (
    <div role="alert" className="mx-auto flex w-full max-w-md flex-col items-center rounded-xl border border-border bg-card p-8 text-center">
      <span className="mb-3 flex size-11 items-center justify-center rounded-full bg-muted text-muted-foreground">
        <TriangleAlert size={20} aria-hidden />
      </span>
      <h1 className="text-lg font-semibold text-foreground">{stale ? "Versi aplikasi sudah diperbarui" : "Terjadi kesalahan pada halaman ini"}</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        {stale
          ? "Muat ulang halaman untuk memakai versi terbaru. Data yang sudah tersimpan tidak hilang."
          : "Halaman gagal ditampilkan. Muat ulang halaman; bila masih terjadi, sampaikan ID kesalahan di bawah kepada administrator."}
      </p>
      {errorId && <p className="mt-3 rounded-md bg-muted px-2 py-1 font-mono text-xs text-muted-foreground">ID kesalahan: {errorId}</p>}
      <Button className="mt-6" onClick={reload}>
        Muat ulang halaman
      </Button>
    </div>
  );
  return fullScreen ? <div className="flex min-h-screen items-center justify-center bg-background p-4">{card}</div> : card;
}
