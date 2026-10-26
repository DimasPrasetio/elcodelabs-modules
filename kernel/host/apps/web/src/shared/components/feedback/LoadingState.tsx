import { Spinner } from "@/shared/components/primitives/spinner";

export function LoadingState() {
  return (
    <div className="flex min-h-screen items-center justify-center gap-2 text-muted-foreground">
      <Spinner />
      <span aria-hidden="true">Memuat…</span>
    </div>
  );
}
