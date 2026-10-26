import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";
import { Textarea } from "@/shared/components/primitives/textarea";
import { FormField } from "@/shared/components/ui/form-field";
import { FieldGrid } from "@/shared/components/ui/form-layout";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { SectionCard } from "@/shared/components/ui/section-card";
import { DetailSkeleton } from "@/shared/components/ui/skeletons";
import { ErrorState } from "@/shared/components/ui/error-state";
import { useSlot } from "@/shared/composition";
import { useResource } from "@/shared/hooks/use-resource";
import { notify } from "@/shared/lib/notify";
import { apiFieldErrors } from "@/shared/services/api-error";
import { LogoCard } from "@/modules/settings/components/LogoCard";
import { fetchCompany, saveCompany } from "@/modules/settings/services/settings.service";
import type { Company, CompanyValues } from "@/modules/settings/types/settings";

const EMPTY: CompanyValues = { name: "", legal_name: "", address: "", phone: "", email: "", tax_id: "", timezone: "Asia/Jakarta" };

/**
 * App-wide settings: the company profile and logo, then whatever other modules add through the
 * "settings:sections" slot (each a <SectionCard>).
 */
export default function SettingsPage() {
  const company = useResource((signal) => fetchCompany(signal), []);
  const [current, setCurrent] = useState<Company | null>(null);
  const [values, setValues] = useState<CompanyValues>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const sections = useSlot("settings:sections");

  useEffect(() => {
    if (!company.data) return;
    setCurrent(company.data);
    const { has_logo: _l, logo_upload: _u, ...rest } = company.data;
    setValues(rest);
  }, [company.data]);

  if (company.error) return <ErrorState message={company.error} onRetry={company.reload} />;
  if (!current) return <DetailSkeleton />;

  const set = (k: keyof CompanyValues) => (e: { target: { value: string } }) => setValues((v) => ({ ...v, [k]: e.target.value }));

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!values.name.trim()) {
      setErrors({ name: "Nama wajib diisi" });
      return;
    }
    setBusy(true);
    setErrors({});
    try {
      setCurrent(await saveCompany(values));
      notify.success("Profil perusahaan disimpan");
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (Object.keys(fields).length) setErrors(fields);
      else notify.error(err);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Page>
      <PageHeader title="Pengaturan" description="Profil perusahaan dipakai di kop dokumen, email, dan laporan." />
      <SectionCard title="Profil perusahaan">
        <form onSubmit={submit} noValidate className="grid gap-4">
          <FieldGrid>
            <FormField label="Nama" required error={errors.name}>
              <Input value={values.name} onChange={set("name")} placeholder="mis. Toko Maju" />
            </FormField>
            <FormField label="Nama legal" error={errors.legal_name} hint="Sesuai akta, mis. PT Toko Maju Sejahtera.">
              <Input value={values.legal_name} onChange={set("legal_name")} />
            </FormField>
            <FormField label="NPWP" error={errors.tax_id} hint="15 atau 16 digit.">
              <Input value={values.tax_id} onChange={set("tax_id")} inputMode="numeric" placeholder="00.000.000.0-000.000" />
            </FormField>
            <FormField label="Telepon" error={errors.phone}>
              <Input value={values.phone} onChange={set("phone")} inputMode="tel" />
            </FormField>
            <FormField label="Email" error={errors.email}>
              <Input type="email" value={values.email} onChange={set("email")} />
            </FormField>
            <FormField label="Zona waktu" error={errors.timezone} hint="Menentukan batas hari untuk laporan.">
              <Input value={values.timezone} onChange={set("timezone")} list="settings-timezones" />
            </FormField>
          </FieldGrid>
          <datalist id="settings-timezones">
            <option value="Asia/Jakarta">WIB</option>
            <option value="Asia/Makassar">WITA</option>
            <option value="Asia/Jayapura">WIT</option>
          </datalist>
          <FormField label="Alamat" error={errors.address}>
            <Textarea value={values.address} onChange={set("address")} rows={3} />
          </FormField>
          <div>
            <Button type="submit" loading={busy}>
              Simpan
            </Button>
          </div>
        </form>
      </SectionCard>
      {current.logo_upload && <LogoCard company={current} onChange={setCurrent} />}
      {sections}
    </Page>
  );
}
