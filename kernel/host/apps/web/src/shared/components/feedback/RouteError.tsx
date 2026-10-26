import type { ReactNode } from "react";
import { isRouteErrorResponse, Link, useRouteError } from "react-router-dom";
import { Button } from "@/shared/components/primitives/button";

function Message({ title, body, action }: { title: string; body: string; action: ReactNode }) {
  return (
    <div role="alert" className="flex min-h-[60vh] flex-col items-center justify-center gap-3 px-4 text-center">
      <h1 className="text-xl font-semibold text-foreground">{title}</h1>
      <p className="max-w-md text-sm text-muted-foreground">{body}</p>
      <div className="mt-2">{action}</div>
    </div>
  );
}

/** An address no module serves. */
export function NotFoundPage() {
  return (
    <Message
      title="Halaman tidak ditemukan"
      body="Alamat ini tidak ada atau sudah dipindahkan."
      action={
        <Button asChild variant="outline">
          <Link to="/">Kembali ke beranda</Link>
        </Button>
      }
    />
  );
}

/**
 * What the router shows when a page throws. A page chunk that fails to load usually means the
 * app was updated while this tab was open: reloading fetches the new version.
 */
export function RouteErrorPage() {
  const error = useRouteError();
  if (isRouteErrorResponse(error) && error.status === 404) return <NotFoundPage />;
  const stale = error instanceof Error && /dynamically imported module|Importing a module script failed|Loading chunk/i.test(error.message);
  return (
    <Message
      title={stale ? "Aplikasi baru saja diperbarui" : "Terjadi kesalahan"}
      body={stale ? "Muat ulang halaman untuk memakai versi terbaru." : "Halaman ini gagal ditampilkan. Coba muat ulang; jika berulang, hubungi admin."}
      action={<Button onClick={() => window.location.reload()}>Muat ulang</Button>}
    />
  );
}
