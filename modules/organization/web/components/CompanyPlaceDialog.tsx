import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { FormField } from "@/shared/components/ui/form-field";
import { apiErrorMessage } from "@/shared/services/api-error";
import { notify } from "@/shared/lib/notify";
import { placeInCompany } from "@/modules/organization/services/organization.service";
import type { Company } from "@/modules/organization/types/organization";

interface CompanyPlaceDialogProps {
  company: Company | null;
  onClose: () => void;
}

/**
 * Places someone across another company, from the home company — how a company whose
 * administrators can no longer get in gets one back. Found by exact email only.
 */
export function CompanyPlaceDialog({ company, onClose }: CompanyPlaceDialogProps) {
  const [email, setEmail] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (company) {
      setEmail("");
      setError(null);
    }
  }, [company]);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!company) return;
    setSaving(true);
    setError(null);
    try {
      await placeInCompany(company.id, email);
      notify.success(`Ditempatkan di seluruh ${company.name}`);
      onClose();
    } catch (err) {
      setError(apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      open={company !== null}
      onClose={() => !saving && onClose()}
      size="sm"
      title={`Tempatkan di ${company?.name ?? ""}`}
      description="Orangnya bisa bekerja di seluruh perusahaan itu; perannya di sana diatur dari dalam perusahaan itu."
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button type="submit" form="company-place-form" loading={saving}>
            Tempatkan
          </Button>
        </>
      }
    >
      <form id="company-place-form" onSubmit={submit} noValidate className="grid gap-4">
        {error && <Notice tone="danger" title="Gagal menempatkan">{error}</Notice>}
        <FormField label="Email pengguna" required hint="Alamat email lengkap akunnya.">
          <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </FormField>
      </form>
    </Modal>
  );
}
