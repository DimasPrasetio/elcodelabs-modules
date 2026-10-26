import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";
import { FormField } from "@/shared/components/ui/form-field";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { SectionCard } from "@/shared/components/ui/section-card";
import { DetailSkeleton } from "@/shared/components/ui/skeletons";
import { ErrorState } from "@/shared/components/ui/error-state";
import { useResource } from "@/shared/hooks/use-resource";
import { emitAppEvent } from "@/shared/app-events";
import { notify } from "@/shared/lib/notify";
import { apiFieldErrors } from "@/shared/services/api-error";
import { fetchProfile, updateProfile } from "@/modules/users/services/users.service";

/** The signed-in user's own profile. Email is changed by an administrator. */
export default function AccountPage() {
  const profile = useResource((signal) => fetchProfile(signal), []);
  const [name, setName] = useState("");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (profile.data) setName(profile.data.name);
  }, [profile.data]);

  if (profile.error) return <ErrorState message={profile.error} onRetry={profile.reload} />;
  if (!profile.data) return <DetailSkeleton />;

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!name.trim()) {
      setError("Nama wajib diisi");
      return;
    }
    setBusy(true);
    setError(undefined);
    try {
      await updateProfile(name.trim());
      notify.success("Profil diperbarui");
      // The name shows in the user menu: whoever shows it reloads it.
      emitAppEvent("profile:changed");
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (fields.name) setError(fields.name);
      else notify.error(err);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Page>
      <PageHeader title="Profil saya" description="Nama yang tampil di aplikasi ini." />
      <SectionCard title="Profil" className="max-w-xl">
        <form onSubmit={submit} noValidate className="grid gap-4">
          <FormField label="Nama" required error={error}>
            <Input value={name} onChange={(e) => setName(e.target.value)} autoComplete="name" />
          </FormField>
          <FormField label="Email" hint="Hubungi administrator untuk mengganti email.">
            <Input value={profile.data.email} readOnly disabled />
          </FormField>
          <div>
            <Button type="submit" loading={busy}>
              Simpan
            </Button>
          </div>
        </form>
      </SectionCard>
    </Page>
  );
}
