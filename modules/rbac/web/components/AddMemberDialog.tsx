import { useEffect, useState } from "react";
import { Search } from "lucide-react";
import { Alert, AlertDescription } from "@/shared/components/primitives/alert";
import { Button } from "@/shared/components/primitives/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/shared/components/primitives/dialog";
import { Input } from "@/shared/components/primitives/input";
import { Spinner } from "@/shared/components/primitives/spinner";
import { apiErrorMessage } from "@/shared/services/api-error";
import { addMember, searchUsers } from "@/modules/rbac/services/rbac.service";
import type { RbacUser } from "@/modules/rbac/types/rbac";

interface AddMemberDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  roleId: number;
  roleName: string;
  /** Users who already hold the role — shown, but not addable again. */
  memberIds: Set<number>;
  onAdded: () => void;
}

/** Finds users by name or email and gives them the role. */
export function AddMemberDialog({ open, onOpenChange, roleId, roleName, memberIds, onAdded }: AddMemberDialogProps) {
  const [q, setQ] = useState("");
  const [results, setResults] = useState<RbacUser[]>([]);
  const [loading, setLoading] = useState(false);
  const [adding, setAdding] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setError(null);
    const t = setTimeout(() => {
      setLoading(true);
      searchUsers(q.trim())
        .then(setResults)
        .catch((err) => setError(apiErrorMessage(err)))
        .finally(() => setLoading(false));
    }, 250);
    return () => clearTimeout(t);
  }, [q, open]);

  async function add(user: RbacUser) {
    setAdding(user.id);
    setError(null);
    try {
      await addMember(roleId, user.id);
      onAdded();
    } catch (err) {
      setError(apiErrorMessage(err));
    } finally {
      setAdding(null);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add to {roleName}</DialogTitle>
          <DialogDescription>Search by name or email.</DialogDescription>
        </DialogHeader>
        <div className="relative">
          <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search users…" aria-label="Search users" className="pl-8" autoFocus />
        </div>
        {error && (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        <ul className="-mx-2 max-h-72 overflow-y-auto">
          {loading && results.length === 0 && (
            <li className="flex justify-center py-6 text-muted-foreground">
              <Spinner />
            </li>
          )}
          {!loading && results.length === 0 && <li className="px-2 py-6 text-center text-sm text-muted-foreground">No users found.</li>}
          {results.map((u) => {
            const member = memberIds.has(u.id);
            return (
              <li key={u.id} className="flex items-center justify-between gap-3 rounded-md px-2 py-2 hover:bg-muted">
                <div className="min-w-0">
                  <div className="truncate text-sm font-medium">{u.name}</div>
                  <div className="truncate text-xs text-muted-foreground">{u.email}</div>
                </div>
                {member ? (
                  <span className="text-xs text-muted-foreground">Already a member</span>
                ) : (
                  <Button size="sm" variant="outline" disabled={adding !== null} onClick={() => add(u)}>
                    {adding === u.id && <Spinner />}
                    Add
                  </Button>
                )}
              </li>
            );
          })}
        </ul>
      </DialogContent>
    </Dialog>
  );
}
