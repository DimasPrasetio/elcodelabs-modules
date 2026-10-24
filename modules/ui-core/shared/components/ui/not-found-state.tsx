import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { FileQuestion } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";

export interface NotFoundStateProps {
  title?: string;
  message?: string;
  backTo?: string;
  backLabel?: string;
  children?: ReactNode;
}

/** 404 for an unknown route or a record that does not exist. */
export function NotFoundState({
  title = "Halaman tidak ditemukan",
  message = "Alamat yang Anda buka tidak ada atau sudah dipindahkan.",
  backTo = "/",
  backLabel = "Kembali ke beranda",
  children,
}: NotFoundStateProps) {
  return (
    <div role="alert" className="mx-auto flex max-w-md flex-col items-center rounded-xl border border-border bg-card p-8 text-center">
      <span className="mb-3 flex size-11 items-center justify-center rounded-full bg-muted text-muted-foreground">
        <FileQuestion size={20} aria-hidden />
      </span>
      <p className="font-display text-3xl font-semibold text-foreground">404</p>
      <h1 className="mt-1 text-lg font-semibold text-foreground">{title}</h1>
      <p className="mt-1 text-sm [overflow-wrap:anywhere] text-muted-foreground">{message}</p>
      {children}
      <Button asChild className="mt-6">
        <Link to={backTo}>{backLabel}</Link>
      </Button>
    </div>
  );
}
