import { useEffect, useState } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { AsyncSearchSelect } from "@/shared/components/ui/async-search-select";
import type { SelectOption, SelectValue } from "@/shared/components/ui/select-parts";
import { FormField } from "@/shared/components/ui/form-field";
import { apiErrorMessage } from "@/shared/services/api-error";
import { notify } from "@/shared/lib/notify";
import { addMember, searchUsers } from "@/modules/rbac/services/rbac.service";

interface AddMemberDialogProps {
  open: boolean;
  onClose: () => void;
  roleId: number;
  roleName: string;
  /** Users who already hold the role — listed but not choosable. */
  memberIds: Set<number>;
  onAdded: () => void;
}

/** Gives the role to a user found by name or email. */
export function AddMemberDialog({ open, onClose, roleId, roleName, memberIds, onAdded }: AddMemberDialogProps) {
  const [userId, setUserId] = useState<SelectValue | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (open) {
      setUserId(null);
      setError(null);
    }
  }, [open]);

  async function load(q: string): Promise<SelectOption[]> {
    const users = await searchUsers(q);
    return users.map((u) => ({
      value: u.id,
      label: u.name,
      description: memberIds.has(u.id) ? `${u.email} · sudah memegang peran ini` : u.email,
      disabled: memberIds.has(u.id),
    }));
  }

  async function add() {
    if (userId === null) {
      setError("Pilih pengguna terlebih dahulu");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await addMember(roleId, Number(userId));
      notify.success(`Peran ${roleName} diberikan`);
      onAdded();
      onClose();
    } catch (err) {
      setError(apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={() => !saving && onClose()}
      size="sm"
      title={`Tambah anggota ${roleName}`}
      description="Cari pengguna berdasarkan nama atau email."
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button onClick={add} loading={saving}>
            Tambah
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        {error && <Notice tone="danger" title="Gagal menambah">{error}</Notice>}
        <FormField label="Pengguna" required>
          <AsyncSearchSelect loadOptions={(q) => load(q)} value={userId} onChange={(v) => setUserId(v)} placeholder="Pilih pengguna…" searchPlaceholder="Cari nama atau email…" />
        </FormField>
      </div>
    </Modal>
  );
}
