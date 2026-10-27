import { useEffect, useState } from "react";
import { Button } from "@/shared/components/primitives/button";
import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { AsyncSearchSelect } from "@/shared/components/ui/async-search-select";
import type { SelectOption, SelectValue } from "@/shared/components/ui/select-parts";
import { FormField } from "@/shared/components/ui/form-field";
import { apiErrorMessage } from "@/shared/services/api-error";
import { notify } from "@/shared/lib/notify";
import { placeUser, searchCandidates } from "@/modules/organization/services/organization.service";
import type { Branch, OrgUser } from "@/modules/organization/types/organization";

interface PlaceUserDialogProps {
  open: boolean;
  onClose: () => void;
  companyName: string;
  branches: Branch[];
  /** Working in one branch: only there (placing across the company needs the whole company). */
  onlyBranch?: number;
  /** Someone to place, chosen already (from "Belum ditempatkan"). */
  user?: OrgUser;
  onPlaced: () => void;
}

/** Places a user in the active company: across it, or in one of its branches. */
export function PlaceUserDialog({ open, onClose, companyName, branches, onlyBranch, user, onPlaced }: PlaceUserDialogProps) {
  const [userId, setUserId] = useState<SelectValue | null>(null);
  const [branchId, setBranchId] = useState("0");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setUserId(user?.id ?? null);
    setBranchId(String(onlyBranch ?? 0));
    setError(null);
  }, [open, user, onlyBranch]);

  async function load(q: string): Promise<SelectOption[]> {
    const users = await searchCandidates(q);
    return users.map((u) => ({ value: u.id, label: u.name, description: u.email }));
  }

  async function place() {
    if (userId === null) {
      setError("Pilih pengguna terlebih dahulu");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await placeUser(Number(userId), Number(branchId));
      notify.success("Pengguna ditempatkan");
      onPlaced();
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
      title={`Tempatkan di ${companyName}`}
      description="Penempatan membuka pintu ke tempat itu; apa yang boleh dilakukan di sana ditentukan perannya."
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button onClick={place} loading={saving}>
            Tempatkan
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        {error && <Notice tone="danger" title="Gagal menempatkan">{error}</Notice>}
        {user ? (
          <FormField label="Pengguna">
            <div className="text-sm">
              <div className="font-medium">{user.name}</div>
              <div className="text-muted-foreground">{user.email}</div>
            </div>
          </FormField>
        ) : (
          <FormField label="Pengguna" required hint="Orang dari perusahaan lain dicari dengan alamat email lengkapnya.">
            <AsyncSearchSelect loadOptions={(q) => load(q)} value={userId} onChange={(v) => setUserId(v)} placeholder="Pilih pengguna…" searchPlaceholder="Nama, atau email lengkap…" />
          </FormField>
        )}
        <FormField label="Di mana">
          <NativeSelect value={branchId} onChange={(e) => setBranchId(e.target.value)} disabled={onlyBranch !== undefined}>
            {onlyBranch === undefined && <NativeSelectOption value="0">Seluruh {companyName}</NativeSelectOption>}
            {branches
              .filter((b) => b.is_active && (onlyBranch === undefined || b.id === onlyBranch))
              .map((b) => (
                <NativeSelectOption key={b.id} value={String(b.id)}>
                  {b.name}
                </NativeSelectOption>
              ))}
          </NativeSelect>
        </FormField>
      </div>
    </Modal>
  );
}
