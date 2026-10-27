import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ArrowLeft, Ban, Eye, MapPin, Pencil, Plus, Power, ShieldOff, Trash2, UserRound } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { ActionMenu } from "@/shared/components/ui/action-menu";
import { DescriptionList } from "@/shared/components/ui/description-list";
import { EmptyState } from "@/shared/components/ui/empty-state";
import { ErrorState } from "@/shared/components/ui/error-state";
import { Notice } from "@/shared/components/ui/notice";
import { Page, PageColumns } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { SectionCard } from "@/shared/components/ui/section-card";
import { DetailSkeleton } from "@/shared/components/ui/skeletons";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import { ConfirmDialog } from "@/shared/components/ui/confirm-dialog";
import { ReasonDialog } from "@/shared/components/ui/reason-dialog";
import { useCan } from "@/shared/access";
import { useResource } from "@/shared/hooks/use-resource";
import { formatDateTime, formatIDR } from "@/shared/lib/format";
import { notify } from "@/shared/lib/notify";
import { AddressDialog } from "@/modules/customers/components/AddressDialog";
import { ContactDialog } from "@/modules/customers/components/ContactDialog";
import { CustomerFormSheet } from "@/modules/customers/components/CustomerFormSheet";
import { CustomerStatusBadge } from "@/modules/customers/components/CustomerStatusBadge";
import {
  deleteContact,
  fetchCustomer,
  holdCustomer,
  releaseCustomer,
  revealTaxID,
  setAddressActive,
  setCustomerActive,
} from "@/modules/customers/services/customers.service";
import { COLLECTOR_LABEL, PARTY_LABEL, TAX_ID_LABEL, countryLabel, termsLabel } from "@/modules/customers/types/customer";
import type { Address, Contact } from "@/modules/customers/types/customer";

export default function CustomerPage() {
  const id = Number(useParams().id);
  const canManage = useCan("customers.manage");
  const detail = useResource((signal) => fetchCustomer(id, signal), [id]);
  const [editing, setEditing] = useState(false);
  const [holding, setHolding] = useState(false);
  const [holdBusy, setHoldBusy] = useState(false);
  const [toggling, setToggling] = useState(false);
  const [revealed, setRevealed] = useState<string | null>(null);
  const [address, setAddress] = useState<Address | "new" | null>(null);
  const [togglingAddress, setTogglingAddress] = useState<Address | null>(null);
  const [contact, setContact] = useState<Contact | "new" | null>(null);
  const [removing, setRemoving] = useState<Contact | null>(null);

  if (detail.error) {
    return (
      <Page>
        <ErrorState message={detail.error} onRetry={detail.reload} />
      </Page>
    );
  }
  if (!detail.data) {
    return (
      <Page>
        <DetailSkeleton />
      </Page>
    );
  }

  const { customer: c, addresses, contacts, can } = detail.data;
  const reload = () => {
    setRevealed(null);
    detail.reload();
  };

  async function reveal() {
    try {
      setRevealed((await revealTaxID(c.id)).tax_id);
    } catch (err) {
      notify.error(err);
    }
  }

  const taxValue =
    c.tax_id_kind === "none" ? (
      "–"
    ) : (
      <span className="inline-flex flex-wrap items-center gap-2">
        <span className="font-mono">{revealed ?? c.tax_id}</span>
        {c.tax_id_masked && !revealed && can.sensitive && (
          <Button variant="link" size="xs" onClick={reveal}>
            <Eye />
            Tampilkan
          </Button>
        )}
      </span>
    );

  const menu = canManage
    ? [
        { key: "edit", label: "Ubah", icon: <Pencil />, onSelect: () => setEditing(true) },
        c.on_hold
          ? {
              key: "release",
              label: "Lepas tahanan",
              icon: <ShieldOff />,
              onSelect: async () => {
                try {
                  await releaseCustomer(c.id);
                  notify.success("Tahanan dilepas");
                  reload();
                } catch (err) {
                  notify.error(err);
                }
              },
            }
          : { key: "hold", label: "Tahan", icon: <Ban />, onSelect: () => setHolding(true) },
        { key: "toggle", label: c.is_active ? "Nonaktifkan" : "Aktifkan", icon: <Power />, destructive: c.is_active, onSelect: () => setToggling(true) },
      ]
    : [];

  return (
    <Page>
      <Link to="/customers" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft size={16} aria-hidden />
        Pelanggan
      </Link>
      <PageHeader
        title={c.name}
        meta={
          <span className="inline-flex items-center gap-2">
            <span className="font-mono">{c.code}</span>
            <CustomerStatusBadge customer={c} />
          </span>
        }
        description={c.legal_name && c.legal_name !== c.name ? c.legal_name : undefined}
        actions={
          canManage && (
            <>
              <Button variant="outline" onClick={() => setEditing(true)}>
                <Pencil />
                Ubah
              </Button>
              <ActionMenu label={`Aksi lain untuk ${c.name}`} items={menu.slice(1)} />
            </>
          )
        }
      />
      {c.on_hold && (
        <Notice tone="warning" title="Pelanggan ditahan">
          {c.hold_reason} — dokumen baru untuk pelanggan ini ditolak sampai tahanan dilepas.
        </Notice>
      )}
      {!c.is_active && <Notice tone="info" title="Pelanggan nonaktif">Dokumen lama tetap tampil; pelanggan ini tidak bisa dipakai di dokumen baru.</Notice>}

      <PageColumns
        main={
          <>
            <SectionCard title="Identitas dan pajak">
              <DescriptionList
                columns={2}
                items={[
                  { label: "Jenis", value: PARTY_LABEL[c.party_type] },
                  { label: "Negara", value: c.is_foreign ? countryLabel(c.country) : "Indonesia" },
                  { label: TAX_ID_LABEL[c.tax_id_kind], value: taxValue },
                  { label: "PKP", value: c.is_pkp ? "Ya" : "Bukan PKP" },
                  { label: "Pemungut PPN", value: COLLECTOR_LABEL[c.vat_collector] },
                  { label: "Email", value: c.email },
                  { label: "Telepon", value: c.phone },
                  { label: "Catatan", value: c.notes, wide: true },
                ]}
              />
            </SectionCard>

            <SectionCard
              title="Alamat"
              actions={
                canManage && (
                  <Button size="sm" variant="outline" onClick={() => setAddress("new")}>
                    <Plus />
                    Alamat
                  </Button>
                )
              }
            >
              {addresses.length === 0 ? (
                <EmptyState icon={MapPin} title="Belum ada alamat" description="Alamat tagih dicetak di faktur; alamat kirim di surat jalan." />
              ) : (
                <ul className="divide-y divide-border">
                  {addresses.map((a) => (
                    <li key={a.id} className="flex items-start justify-between gap-3 py-3 first:pt-0 last:pb-0">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="font-medium">{a.label}</span>
                          {a.is_default_billing && <StatusBadge tone="info">Tagih utama</StatusBadge>}
                          {a.is_default_shipping && <StatusBadge tone="info">Kirim utama</StatusBadge>}
                          {!a.is_active && <StatusBadge tone="muted">Nonaktif</StatusBadge>}
                        </div>
                        <p className="mt-1 text-sm text-muted-foreground">
                          {[a.line, a.city, a.province, a.postal_code].filter(Boolean).join(", ")}
                          {a.country !== "ID" && `, ${countryLabel(a.country)}`}
                        </p>
                        {a.nitku && <p className="mt-0.5 font-mono text-xs text-muted-foreground">NITKU {a.nitku}</p>}
                      </div>
                      {canManage && (
                        <ActionMenu
                          variant="ghost"
                          size="icon-sm"
                          label={`Aksi untuk alamat ${a.label}`}
                          items={[
                            { key: "edit", label: "Ubah", icon: <Pencil />, onSelect: () => setAddress(a) },
                            { key: "toggle", label: a.is_active ? "Nonaktifkan" : "Aktifkan", icon: <Power />, destructive: a.is_active, onSelect: () => setTogglingAddress(a) },
                          ]}
                        />
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </SectionCard>

            <SectionCard
              title="Kontak"
              actions={
                canManage && (
                  <Button size="sm" variant="outline" onClick={() => setContact("new")}>
                    <Plus />
                    Kontak
                  </Button>
                )
              }
            >
              {contacts.length === 0 ? (
                <EmptyState icon={UserRound} title="Belum ada kontak" description="Orang yang dihubungi di pihak pelanggan." />
              ) : (
                <ul className="divide-y divide-border">
                  {contacts.map((ct) => (
                    <li key={ct.id} className="flex items-start justify-between gap-3 py-3 first:pt-0 last:pb-0">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="font-medium">{ct.name}</span>
                          {ct.title && <span className="text-sm text-muted-foreground">{ct.title}</span>}
                          {ct.is_primary && <StatusBadge tone="info">Utama</StatusBadge>}
                        </div>
                        <p className="mt-1 text-sm text-muted-foreground">{[ct.phone, ct.email].filter(Boolean).join(" · ") || "–"}</p>
                      </div>
                      {canManage && (
                        <ActionMenu
                          variant="ghost"
                          size="icon-sm"
                          label={`Aksi untuk kontak ${ct.name}`}
                          items={[
                            { key: "edit", label: "Ubah", icon: <Pencil />, onSelect: () => setContact(ct) },
                            { key: "delete", label: "Hapus", icon: <Trash2 />, destructive: true, onSelect: () => setRemoving(ct) },
                          ]}
                        />
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </SectionCard>
          </>
        }
        aside={
          <SectionCard title="Dagang">
            <DescriptionList
              columns={1}
              items={[
                { label: "Termin pembayaran", value: termsLabel(c.payment_days) },
                { label: "Batas kredit", value: c.credit_limit == null ? "Tanpa batas" : formatIDR(Number(c.credit_limit)) },
                { label: "Diperbarui", value: formatDateTime(c.updated_at) },
              ]}
            />
          </SectionCard>
        }
      />

      <CustomerFormSheet open={editing} onClose={() => setEditing(false)} customer={c} canCredit={can.credit} onSaved={reload} />
      <ReasonDialog
        open={holding}
        onClose={() => setHolding(false)}
        title={`Tahan ${c.name}?`}
        description="Dokumen baru untuk pelanggan ini ditolak sampai tahanan dilepas. Dokumen yang sudah ada tidak berubah."
        confirmLabel="Tahan pelanggan"
        placeholder="mis. tunggakan lewat 90 hari"
        submitting={holdBusy}
        onSubmit={async (reason) => {
          setHoldBusy(true);
          try {
            await holdCustomer(c.id, reason);
            notify.success("Pelanggan ditahan");
            setHolding(false);
            reload();
          } catch (err) {
            notify.error(err);
          } finally {
            setHoldBusy(false);
          }
        }}
      />
      <ConfirmDialog
        open={toggling}
        onOpenChange={setToggling}
        tone={c.is_active ? "danger" : "primary"}
        title={c.is_active ? `Nonaktifkan ${c.name}?` : `Aktifkan ${c.name}?`}
        description={
          c.is_active
            ? "Pelanggan tidak bisa dipakai di dokumen baru. Dokumen lama tetap menampilkannya, dan pelanggan bisa diaktifkan lagi."
            : "Pelanggan bisa dipakai lagi di dokumen baru."
        }
        confirmLabel={c.is_active ? "Nonaktifkan" : "Aktifkan"}
        onConfirm={async () => {
          await setCustomerActive(c.id, !c.is_active);
          notify.success(c.is_active ? "Pelanggan dinonaktifkan" : "Pelanggan diaktifkan");
          reload();
        }}
      />
      <AddressDialog
        open={address !== null}
        onClose={() => setAddress(null)}
        customerId={c.id}
        hasTaxID={c.tax_id_kind === "npwp" || c.tax_id_kind === "nik"}
        address={address === "new" || address === null ? undefined : address}
        first={addresses.length === 0}
        onSaved={reload}
      />
      <ConfirmDialog
        open={togglingAddress !== null}
        onOpenChange={(open) => !open && setTogglingAddress(null)}
        tone={togglingAddress?.is_active ? "danger" : "primary"}
        title={togglingAddress?.is_active ? `Nonaktifkan alamat ${togglingAddress?.label}?` : `Aktifkan alamat ${togglingAddress?.label}?`}
        description={togglingAddress?.is_active ? "Dokumen lama tetap memakai salinannya. Tanda alamat utama dilepas." : "Alamat bisa dipilih lagi di dokumen baru."}
        confirmLabel={togglingAddress?.is_active ? "Nonaktifkan" : "Aktifkan"}
        onConfirm={async () => {
          if (!togglingAddress) return;
          await setAddressActive(c.id, togglingAddress.id, !togglingAddress.is_active);
          notify.success("Alamat disimpan");
          reload();
        }}
      />
      <ContactDialog
        open={contact !== null}
        onClose={() => setContact(null)}
        customerId={c.id}
        contact={contact === "new" || contact === null ? undefined : contact}
        first={contacts.length === 0}
        onSaved={reload}
      />
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
        tone="danger"
        title={`Hapus kontak ${removing?.name}?`}
        confirmLabel="Hapus"
        onConfirm={async () => {
          if (!removing) return;
          await deleteContact(c.id, removing.id);
          notify.success("Kontak dihapus");
          reload();
        }}
      />
    </Page>
  );
}
