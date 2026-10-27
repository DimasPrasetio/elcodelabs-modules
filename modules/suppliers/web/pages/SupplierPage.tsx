import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ArrowLeft, Ban, Eye, Landmark, MapPin, Pencil, Plus, Power, ShieldOff, Trash2, UserRound } from "lucide-react";
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
import { formatDateTime } from "@/shared/lib/format";
import { notify } from "@/shared/lib/notify";
import { AddressDialog } from "@/modules/suppliers/components/AddressDialog";
import { BankAccountDialog } from "@/modules/suppliers/components/BankAccountDialog";
import { ContactDialog } from "@/modules/suppliers/components/ContactDialog";
import { SupplierFormSheet } from "@/modules/suppliers/components/SupplierFormSheet";
import { SupplierStatusBadge } from "@/modules/suppliers/components/SupplierStatusBadge";
import {
  deleteContact,
  fetchSupplier,
  holdSupplier,
  releaseSupplier,
  revealSensitive,
  setBankAccountActive,
  setAddressActive,
  setSupplierActive,
} from "@/modules/suppliers/services/suppliers.service";
import { PARTY_LABEL, TAX_ID_LABEL, countryLabel, termsLabel } from "@/modules/suppliers/types/supplier";
import type { Address, BankAccount, Contact, Revealed } from "@/modules/suppliers/types/supplier";

export default function SupplierPage() {
  const id = Number(useParams().id);
  const canManage = useCan("suppliers.manage");
  const detail = useResource((signal) => fetchSupplier(id, signal), [id]);
  const [editing, setEditing] = useState(false);
  const [holding, setHolding] = useState(false);
  const [holdBusy, setHoldBusy] = useState(false);
  const [toggling, setToggling] = useState(false);
  const [revealed, setRevealed] = useState<Revealed | null>(null);
  const [bank, setBank] = useState<BankAccount | "new" | null>(null);
  const [togglingBank, setTogglingBank] = useState<BankAccount | null>(null);
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

  const { supplier: c, addresses, bank_accounts: banks, contacts, can } = detail.data;
  const reload = () => {
    setRevealed(null);
    detail.reload();
  };

  // One audited read shows the whole tax ID and every account number.
  async function reveal(): Promise<Revealed | null> {
    if (revealed) return revealed;
    try {
      const r = await revealSensitive(c.id);
      setRevealed(r);
      return r;
    } catch (err) {
      notify.error(err);
      return null;
    }
  }

  // Changing an account starts from its whole number.
  async function editBank(b: BankAccount) {
    const r = await reveal();
    const whole = r?.bank_accounts.find((x) => x.id === b.id);
    if (whole) setBank(whole);
  }

  const accountNumber = (b: BankAccount) => revealed?.bank_accounts.find((x) => x.id === b.id)?.account_number ?? b.account_number;

  const taxValue =
    c.tax_id_kind === "none" ? (
      "–"
    ) : (
      <span className="inline-flex flex-wrap items-center gap-2">
        <span className="font-mono">{revealed?.tax_id ?? c.tax_id}</span>
        {c.tax_id_masked && !revealed && can.sensitive && (
          <Button variant="link" size="xs" onClick={() => void reveal()}>
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
                  await releaseSupplier(c.id);
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
      <Link to="/suppliers" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft size={16} aria-hidden />
        Pemasok
      </Link>
      <PageHeader
        title={c.name}
        meta={
          <span className="inline-flex items-center gap-2">
            <span className="font-mono">{c.code}</span>
            <SupplierStatusBadge supplier={c} />
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
        <Notice tone="warning" title="Pemasok ditahan">
          {c.hold_reason} — dokumen pembelian baru untuk pemasok ini ditolak sampai tahanan dilepas.
        </Notice>
      )}
      {!c.is_active && <Notice tone="info" title="Pemasok nonaktif">Dokumen lama tetap tampil; pemasok ini tidak bisa dipakai di dokumen baru.</Notice>}

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
                <EmptyState icon={MapPin} title="Belum ada alamat" description="Alamat pemasok untuk dokumen pembelian dan bukti potong pajak." />
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
              title="Rekening bank"
              description={can.sensitive ? undefined : "Nomor rekening tersamar. Rekening diubah oleh pemegang izin suppliers.sensitive."}
              actions={
                can.sensitive && (
                  <>
                    {banks.length > 0 && !revealed && (
                      <Button size="sm" variant="ghost" onClick={() => void reveal()}>
                        <Eye />
                        Tampilkan
                      </Button>
                    )}
                    <Button size="sm" variant="outline" onClick={() => setBank("new")}>
                      <Plus />
                      Rekening
                    </Button>
                  </>
                )
              }
            >
              {banks.length === 0 ? (
                <EmptyState icon={Landmark} title="Belum ada rekening" description="Rekening tujuan pembayaran untuk pemasok ini." />
              ) : (
                <ul className="divide-y divide-border">
                  {banks.map((b) => (
                    <li key={b.id} className="flex items-start justify-between gap-3 py-3 first:pt-0 last:pb-0">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="font-medium">{b.bank_name}</span>
                          {b.bank_code && <span className="text-xs text-muted-foreground">{b.bank_code}</span>}
                          {b.is_default && <StatusBadge tone="info">Utama</StatusBadge>}
                          {!b.is_active && <StatusBadge tone="muted">Nonaktif</StatusBadge>}
                        </div>
                        <p className="mt-1 text-sm">
                          <span className="font-mono">{accountNumber(b)}</span>
                          <span className="text-muted-foreground"> a.n. {b.account_holder}</span>
                        </p>
                      </div>
                      {can.sensitive && (
                        <ActionMenu
                          variant="ghost"
                          size="icon-sm"
                          label={`Aksi untuk rekening ${b.bank_name}`}
                          items={[
                            { key: "edit", label: "Ubah", icon: <Pencil />, onSelect: () => void editBank(b) },
                            { key: "toggle", label: b.is_active ? "Nonaktifkan" : "Aktifkan", icon: <Power />, destructive: b.is_active, onSelect: () => setTogglingBank(b) },
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
                <EmptyState icon={UserRound} title="Belum ada kontak" description="Orang yang dihubungi di pihak pemasok." />
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
                { label: "Diperbarui", value: formatDateTime(c.updated_at) },
              ]}
            />
          </SectionCard>
        }
      />

      <SupplierFormSheet open={editing} onClose={() => setEditing(false)} supplier={c} onSaved={reload} />
      <ReasonDialog
        open={holding}
        onClose={() => setHolding(false)}
        title={`Tahan ${c.name}?`}
        description="Dokumen baru untuk pemasok ini ditolak sampai tahanan dilepas. Dokumen yang sudah ada tidak berubah."
        confirmLabel="Tahan pemasok"
        placeholder="mis. kualitas barang sedang diperiksa"
        submitting={holdBusy}
        onSubmit={async (reason) => {
          setHoldBusy(true);
          try {
            await holdSupplier(c.id, reason);
            notify.success("Pemasok ditahan");
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
            ? "Pemasok tidak bisa dipakai di dokumen baru. Dokumen lama tetap menampilkannya, dan pemasok bisa diaktifkan lagi."
            : "Pemasok bisa dipakai lagi di dokumen baru."
        }
        confirmLabel={c.is_active ? "Nonaktifkan" : "Aktifkan"}
        onConfirm={async () => {
          await setSupplierActive(c.id, !c.is_active);
          notify.success(c.is_active ? "Pemasok dinonaktifkan" : "Pemasok diaktifkan");
          reload();
        }}
      />
      <AddressDialog
        open={address !== null}
        onClose={() => setAddress(null)}
        supplierId={c.id}
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
      <BankAccountDialog
        open={bank !== null}
        onClose={() => setBank(null)}
        supplierId={c.id}
        supplierName={c.legal_name || c.name}
        account={bank === "new" || bank === null ? undefined : bank}
        first={banks.length === 0}
        onSaved={reload}
      />
      <ConfirmDialog
        open={togglingBank !== null}
        onOpenChange={(open) => !open && setTogglingBank(null)}
        tone={togglingBank?.is_active ? "danger" : "primary"}
        title={togglingBank?.is_active ? `Nonaktifkan rekening ${togglingBank?.bank_name}?` : `Aktifkan rekening ${togglingBank?.bank_name}?`}
        description={togglingBank?.is_active ? "Pembayaran baru tidak lagi dikirim ke rekening ini. Pembayaran lama tetap menyebutnya." : "Rekening bisa dipakai lagi untuk pembayaran."}
        confirmLabel={togglingBank?.is_active ? "Nonaktifkan" : "Aktifkan"}
        onConfirm={async () => {
          if (!togglingBank) return;
          await setBankAccountActive(c.id, togglingBank.id, !togglingBank.is_active);
          notify.success("Rekening disimpan");
          reload();
        }}
      />
      <ContactDialog
        open={contact !== null}
        onClose={() => setContact(null)}
        supplierId={c.id}
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
