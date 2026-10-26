import { useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/shared/components/primitives/card";
import { Spinner } from "@/shared/components/primitives/spinner";
import { useHealthStore } from "@/modules/dashboard/stores/health.store";

export default function DashboardPage() {
  const { status, result, error, check } = useHealthStore();

  useEffect(() => {
    check();
  }, [check]);

  return (
    <div>
      <h1 className="font-display text-2xl font-semibold">Dashboard</h1>
      <Card className="mt-4">
        <CardHeader>
          <CardTitle>Kesehatan API</CardTitle>
          <CardDescription>GET /api/v1/health</CardDescription>
        </CardHeader>
        <CardContent>
          {status === "loading" && <Spinner />}
          {status === "error" && <p className="text-sm text-destructive">{error}</p>}
          {status === "success" && (
            <pre className="font-mono text-sm">{JSON.stringify(result, null, 2)}</pre>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
