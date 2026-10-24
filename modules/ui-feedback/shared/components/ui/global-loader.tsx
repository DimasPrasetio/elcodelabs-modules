import { useRequestsPending } from "@/shared/lib/request-tracker";

/** A thin bar at the top of the viewport while requests are running. */
export function GlobalLoader() {
  const active = useRequestsPending();
  if (!active) return null;
  return (
    <div role="progressbar" aria-label="Memuat data" className="pointer-events-none fixed inset-x-0 top-0 z-[9999] h-[3px] overflow-hidden bg-primary/10">
      <div aria-hidden className="h-full w-1/3 animate-loader-slide rounded-r-[2px] bg-primary shadow-(--shadow-glow)" />
    </div>
  );
}
