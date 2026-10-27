import { useState } from "react";
import type { ReactNode } from "react";
import { Download, Pencil, Plus, Printer, Trash2 } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import { Input } from "@/shared/components/primitives/input";
import { Label } from "@/shared/components/primitives/label";
import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { RadioGroup, RadioGroupItem } from "@/shared/components/primitives/radio-group";
import { Switch } from "@/shared/components/primitives/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/shared/components/primitives/tabs";
import { Textarea } from "@/shared/components/primitives/textarea";
import { DescriptionList } from "@/shared/components/ui/description-list";
import { EmptyState } from "@/shared/components/ui/empty-state";
import { ErrorState } from "@/shared/components/ui/error-state";
import { FormSheet } from "@/shared/components/ui/form-sheet";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { SectionCard } from "@/shared/components/ui/section-card";
import { CardSkeleton, TableSkeleton } from "@/shared/components/ui/skeletons";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import { SummaryCard } from "@/shared/components/ui/summary-card";
import { DocNumber, Money, Qty } from "@/shared/components/ui/values";
import { ActionMenu } from "@/shared/components/ui/action-menu";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn, DataTableSort } from "@/shared/components/ui/data-table";
import { Pagination } from "@/shared/components/ui/pagination";
import { SearchInput } from "@/shared/components/ui/search-input";
import { Toolbar } from "@/shared/components/ui/toolbar";
import { AsyncSearchSelect } from "@/shared/components/ui/async-search-select";
import { DateRangeFields } from "@/shared/components/ui/date-range-fields";
import { FieldHint } from "@/shared/components/ui/field-hint";
import { FileDropzone } from "@/shared/components/ui/file-dropzone";
import { FieldGrid, FormSection } from "@/shared/components/ui/form-layout";
import { FormField } from "@/shared/components/ui/form-field";
import { HierarchicalSelect } from "@/shared/components/ui/hierarchical-select";
import type { TreeOption } from "@/shared/components/ui/hierarchical-select";
import { KeyValueEditor } from "@/shared/components/ui/key-value-editor";
import type { KeyValueRow } from "@/shared/components/ui/key-value-editor";
import { LineItemsEditor } from "@/shared/components/ui/line-items-editor";
import { MultiToggle } from "@/shared/components/ui/multi-toggle";
import { PasswordInput } from "@/shared/components/ui/password-input";
import { DigitsInput } from "@/shared/components/ui/digits-input";
import { AddressFields, EMPTY_ADDRESS } from "@/shared/components/ui/address-fields";
import { SearchSelect } from "@/shared/components/ui/search-select";
import type { SelectOption, SelectValue } from "@/shared/components/ui/select-parts";
import { Segmented } from "@/shared/components/ui/segmented";
import { Stepper } from "@/shared/components/ui/stepper";
import { ConfirmDialog } from "@/shared/components/ui/confirm-dialog";
import { ReasonDialog } from "@/shared/components/ui/reason-dialog";
import { notify } from "@/shared/lib/notify";
import type { Tone } from "@/shared/lib/tone";

interface Order {
  id: number;
  number: string;
  date: string;
  customer: string;
  status: { label: string; tone: Tone };
  total: number;
}

const ORDERS: Order[] = [
  { id: 1, number: "SO-2026-00012", date: "2026-10-16", customer: "Toko Maju Jaya", status: { label: "Dikirim", tone: "info" }, total: 12450000 },
  { id: 2, number: "SO-2026-00011", date: "2026-10-15", customer: "CV Sumber Rejeki", status: { label: "Selesai", tone: "success" }, total: 3275000 },
  { id: 3, number: "SO-2026-00010", date: "2026-10-15", customer: "PT Sentosa Abadi", status: { label: "Jatuh tempo", tone: "warning" }, total: 48900000 },
  { id: 4, number: "SO-2026-00009", date: "2026-10-14", customer: "UD Berkah", status: { label: "Draf", tone: "neutral" }, total: 875000 },
  { id: 5, number: "SO-2026-00008", date: "2026-10-13", customer: "Toko Makmur", status: { label: "Dibatalkan", tone: "muted" }, total: 1500000 },
  { id: 6, number: "SO-2026-00007", date: "2026-10-12", customer: "PT Cahaya Timur", status: { label: "Gagal bayar", tone: "danger" }, total: 6200000 },
];

const CUSTOMERS: SelectOption[] = ORDERS.map((o) => ({ value: o.id, label: o.customer, description: `Pelanggan #${o.id}` }));

const LOCATIONS: TreeOption[] = [
  { value: "gd", label: "Gudang Utama", depth: 0, isGroup: true },
  { value: "gd-a", label: "Rak A", depth: 1, path: ["Gudang Utama"] },
  { value: "gd-a1", label: "Baris 1", depth: 2, path: ["Gudang Utama", "Rak A"] },
  { value: "gd-a2", label: "Baris 2", depth: 2, path: ["Gudang Utama", "Rak A"] },
  { value: "gd-b", label: "Rak B", depth: 1, path: ["Gudang Utama"] },
  { value: "tk", label: "Toko Depan", depth: 0 },
];

interface Line {
  key: number;
  item: string;
  qty: string;
  price: string;
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <SectionCard title={title}>
      <div className="flex flex-col gap-5">{children}</div>
    </SectionCard>
  );
}

function Row({ children }: { children: ReactNode }) {
  return <div className="flex flex-wrap items-center gap-3">{children}</div>;
}

export default function GalleryPage() {
  const [sort, setSort] = useState<DataTableSort>({ key: "date", dir: "desc" });
  const [selected, setSelected] = useState<Set<string | number>>(new Set());
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [customer, setCustomer] = useState<SelectValue | null>(null);
  const [asyncCustomer, setAsyncCustomer] = useState<SelectValue | null>(null);
  const [location, setLocation] = useState<SelectValue | null>("gd-a2");
  const [view, setView] = useState("all");
  const [statuses, setStatuses] = useState<string[]>(["draft"]);
  const [range, setRange] = useState({ from: "2026-10-01", to: "2026-10-16" });
  const [kv, setKv] = useState<KeyValueRow[]>([{ key: "NPWP", value: "01.234.567.8-901.000" }]);
  const [lines, setLines] = useState<Line[]>([{ key: 1, item: "Beras 5 kg", qty: "10", price: "68000" }]);
  const [modal, setModal] = useState(false);
  const [sheet, setSheet] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [reason, setReason] = useState(false);
  const [idNumber, setIdNumber] = useState("3201234567890001");
  const [address, setAddress] = useState({ ...EMPTY_ADDRESS, line: "Jl. Asia Afrika No. 8", city: "Kota Bandung", province: "Jawa Barat", postal_code: "40111" });

  const columns: DataTableColumn<Order>[] = [
    { header: "Nomor", sortKey: "number", render: (o) => <DocNumber value={o.number} copyable /> },
    { header: "Tanggal", sortKey: "date", render: (o) => <span className="tabular whitespace-nowrap">{o.date}</span> },
    { header: "Pelanggan", render: (o) => o.customer },
    { header: "Status", render: (o) => <StatusBadge tone={o.status.tone}>{o.status.label}</StatusBadge> },
    { header: "Total", align: "right", sortKey: "total", render: (o) => <Money value={o.total} /> },
    {
      header: "",
      id: "actions",
      width: 56,
      align: "right",
      render: (o) => (
        <ActionMenu
          variant="ghost"
          size="icon-sm"
          label={`Aksi ${o.number}`}
          items={[
            { key: "edit", label: "Ubah", icon: <Pencil />, onSelect: () => setSheet(true) },
            { key: "print", label: "Cetak", icon: <Printer />, onSelect: () => notify.info("Mencetak…") },
            { key: "cancel", label: "Batalkan", icon: <Trash2 />, destructive: true, onSelect: () => setReason(true) },
          ]}
        />
      ),
    },
  ];

  return (
    <Page>
      <PageHeader
        title="Galeri UI"
        description="Setiap komponen kit ui-core, ui-data, ui-forms, dan ui-feedback di bawah tema proyek ini."
        meta={<StatusBadge tone="info">Pengembang</StatusBadge>}
        actions={
          <>
            <ActionMenu items={[{ key: "export", label: "Ekspor", icon: <Download />, onSelect: () => notify.success("Ekspor dimulai") }]} />
            <Button onClick={() => setSheet(true)}>
              <Plus />
              Order baru
            </Button>
          </>
        }
      />

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <SummaryCard label="Penjualan hari ini" value="Rp 12.450.000" delta={{ text: "+12% vs kemarin", tone: "up" }} />
        <SummaryCard label="Order berjalan" value="128" hint="12 draf" tone="success" />
        <SummaryCard label="Jatuh tempo minggu ini" value="5" tone="warning" onClick={() => notify.info("Membuka daftar jatuh tempo")} />
        <SummaryCard label="Retur" value="2" tone="danger" delta={{ text: "−1 vs minggu lalu", tone: "down" }} />
      </div>

      <section className="overflow-hidden rounded-xl border border-border bg-card">
        <Toolbar
          plain
          search={<SearchInput value={query} onSearch={setQuery} placeholder="Cari nomor / pelanggan…" />}
          primary={<Segmented value={view} onChange={setView} options={[{ value: "all", label: "Semua", count: 128 }, { value: "draft", label: "Draf", count: 12 }, { value: "due", label: "Jatuh tempo", count: 5 }]} />}
          more={<DateRangeFields from={range.from} to={range.to} onChange={setRange} />}
          moreActiveCount={1}
          active={[{ key: "cust", label: "Pelanggan: Toko Maju", onRemove: () => notify.info("Filter dihapus") }]}
          onClearAll={() => notify.info("Semua filter dihapus")}
        />
        <DataTable
          columns={columns}
          rows={ORDERS}
          rowKey={(o) => o.id}
          sort={sort}
          onSortChange={setSort}
          selectable
          selectedKeys={selected}
          onSelectionChange={setSelected}
          bulkActions={() => (
            <Button size="sm" variant="outline" onClick={() => notify.success("Label dicetak")}>
              <Printer /> Cetak label
            </Button>
          )}
          onRowClick={() => setSheet(true)}
          rowLabel={(o) => `Buka ${o.number}`}
          columnsKey="gallery-orders"
          caption="Contoh daftar order"
        />
        <Pagination page={page} limit={25} total={128} onPageChange={setPage} onLimitChange={() => undefined} />
      </section>

      <Section title="Tombol">
        <Row>
          <Button>Utama</Button>
          <Button variant="outline">Sekunder</Button>
          <Button variant="ghost">Ghost</Button>
          <Button variant="destructive">Hapus</Button>
          <Button variant="link">Tautan</Button>
          <Button loading>Menyimpan</Button>
          <Button disabled>Nonaktif</Button>
        </Row>
        <Row>
          <Button size="lg">Besar</Button>
          <Button>Sedang</Button>
          <Button size="sm">Kecil</Button>
          <Button size="xs">Mini</Button>
          <Button size="icon" variant="outline" aria-label="Tambah">
            <Plus />
          </Button>
          <Button size="icon-sm" variant="ghost" aria-label="Cetak">
            <Printer />
          </Button>
        </Row>
      </Section>

      <Section title="Kontrol isian">
        <FieldGrid columns={3}>
          <FormField label="Nama pelanggan" required hint="Seperti di faktur pajak.">
            <Input placeholder="mis. Toko Maju Jaya" />
          </FormField>
          <FormField label="Email" error="Email tidak valid">
            <Input defaultValue="toko@maju" />
          </FormField>
          <FormField label="Kata sandi">
            <PasswordInput defaultValue="rahasia123" />
          </FormField>
          <FormField label="Nomor identitas" hint="Hanya angka; dikelompokkan saat tidak diketik.">
            <DigitsInput value={idNumber} onChange={setIdNumber} maxDigits={16} groups={[4, 4, 4, 4]} />
          </FormField>
          <FormField label="Pelanggan (cari)">
            <SearchSelect options={CUSTOMERS} value={customer} onChange={setCustomer} allowClear />
          </FormField>
          <FormField label="Pelanggan (server)" labelAside={<FieldHint>Dicari di server saat Anda mengetik, minimal 2 huruf.</FieldHint>}>
            <AsyncSearchSelect
              value={asyncCustomer}
              onChange={setAsyncCustomer}
              minChars={2}
              loadOptions={(q) =>
                new Promise((resolve) => setTimeout(() => resolve(CUSTOMERS.filter((c) => c.label.toLowerCase().includes(q.toLowerCase()))), 300))
              }
            />
          </FormField>
          <FormField label="Lokasi">
            <HierarchicalSelect options={LOCATIONS} value={location} onChange={setLocation} />
          </FormField>
          <FormField label="Satuan">
            <NativeSelect defaultValue="pcs">
              <NativeSelectOption value="pcs">Pcs</NativeSelectOption>
              <NativeSelectOption value="box">Box</NativeSelectOption>
              <NativeSelectOption value="kg">Kg</NativeSelectOption>
            </NativeSelect>
          </FormField>
          <FormField label="Catatan">
            <Textarea placeholder="Opsional" />
          </FormField>
          <FormField label="Nonaktif">
            <Input disabled defaultValue="Tidak bisa diubah" />
          </FormField>
        </FieldGrid>
        <Row>
          <label className="flex items-center gap-2 text-sm">
            <Checkbox defaultChecked /> Kirim email konfirmasi
          </label>
          <label className="flex items-center gap-2 text-sm">
            <Switch defaultChecked /> Aktif
          </label>
          <RadioGroup defaultValue="tunai" className="flex gap-4">
            {["tunai", "transfer", "kredit"].map((v) => (
              <div key={v} className="flex items-center gap-2">
                <RadioGroupItem value={v} id={`pay-${v}`} />
                <Label htmlFor={`pay-${v}`} className="capitalize">
                  {v}
                </Label>
              </div>
            ))}
          </RadioGroup>
        </Row>
        <Row>
          <MultiToggle
            aria-label="Status"
            value={statuses}
            onChange={setStatuses}
            options={[
              { value: "draft", label: "Draf" },
              { value: "sent", label: "Dikirim" },
              { value: "done", label: "Selesai" },
            ]}
          />
        </Row>
      </Section>

      <Section title="Alamat">
        <AddressFields
          value={address}
          onChange={setAddress}
          countries={[
            { value: "ID", label: "Indonesia" },
            { value: "SG", label: "Singapura" },
            { value: "MY", label: "Malaysia" },
          ]}
          errors={address.postal_code ? {} : { postal_code: "Kode pos wajib diisi" }}
          required={["line", "postal_code"]}
        />
      </Section>

      <Section title="Status dan pesan">
        <Row>
          {(["neutral", "info", "warning", "success", "danger", "muted"] as Tone[]).map((t) => (
            <StatusBadge key={t} tone={t}>
              {{ neutral: "Draf", info: "Berjalan", warning: "Perlu perhatian", success: "Selesai", danger: "Masalah", muted: "Dibatalkan" }[t]}
            </StatusBadge>
          ))}
        </Row>
        <div className="grid gap-3 md:grid-cols-2">
          <Notice tone="info" title="Info">Stok dihitung ulang setiap malam.</Notice>
          <Notice tone="success" title="Tersimpan">Order SO-2026-00012 sudah disimpan.</Notice>
          <Notice tone="warning" title="Perlu perhatian">5 order jatuh tempo minggu ini.</Notice>
          <ErrorState message="Database tidak dapat dihubungi" onRetry={() => notify.info("Mencoba lagi")} />
        </div>
        <Row>
          <Button variant="outline" onClick={() => notify.success("Order disimpan")}>Toast sukses</Button>
          <Button variant="outline" onClick={() => notify.warning("Stok hampir habis")}>Toast peringatan</Button>
          <Button variant="outline" onClick={() => notify.error("Email atau kata sandi salah")}>Toast galat</Button>
        </Row>
      </Section>

      <div className="grid gap-6 xl:grid-cols-2">
        <SectionCard title="Rincian" actions={<Button size="sm" variant="outline">Ubah</Button>}>
          <DescriptionList
            columns={2}
            items={[
              { label: "Nomor", value: <DocNumber value="SO-2026-00012" copyable /> },
              { label: "Total", value: <Money value={12450000} /> },
              { label: "Jumlah", value: <Qty value={1250.5} decimals={2} unit="kg" /> },
              { label: "Saldo", value: <Money value={-1250000} accounting /> },
              { label: "Catatan", value: "", wide: true },
            ]}
          />
        </SectionCard>
        <SectionCard title="Tahapan">
          <div className="flex flex-col gap-6">
            <Stepper current="sent" steps={[{ key: "draft", label: "Draf" }, { key: "confirmed", label: "Dikonfirmasi" }, { key: "sent", label: "Dikirim" }, { key: "done", label: "Selesai" }]} />
            <Stepper stopped current="confirmed" steps={[{ key: "draft", label: "Draf" }, { key: "confirmed", label: "Dibatalkan" }, { key: "sent", label: "Dikirim" }]} />
            <Tabs defaultValue="a">
              <TabsList>
                <TabsTrigger value="a">Ringkasan</TabsTrigger>
                <TabsTrigger value="b">Riwayat</TabsTrigger>
              </TabsList>
              <TabsContent value="a" className="pt-3 text-sm text-muted-foreground">Isi tab ringkasan.</TabsContent>
              <TabsContent value="b" className="pt-3 text-sm text-muted-foreground">Isi tab riwayat.</TabsContent>
            </Tabs>
          </div>
        </SectionCard>
      </div>

      <SectionCard title="Pola formulir">
        <FormSection title="Baris barang" description="Enter di baris terakhir menambah baris.">
          <LineItemsEditor
            lines={lines}
            getKey={(l) => l.key}
            onAdd={() => setLines((ls) => [...ls, { key: Date.now(), item: "", qty: "1", price: "0" }])}
            onRemove={(i) => setLines((ls) => ls.filter((_, j) => j !== i))}
            footer={
              <span className="text-sm font-medium">
                Total <Money value={lines.reduce((s, l) => s + Number(l.qty || 0) * Number(l.price || 0), 0)} />
              </span>
            }
            columns={[
              { id: "item", header: "Barang", render: (l, i) => <Input aria-label={`Barang baris ${i + 1}`} value={l.item} onChange={(e) => setLines((ls) => ls.map((x) => (x.key === l.key ? { ...x, item: e.target.value } : x)))} /> },
              { id: "qty", header: "Qty", width: "7rem", align: "right", render: (l, i) => <Input aria-label={`Qty baris ${i + 1}`} inputMode="decimal" className="text-right" value={l.qty} onChange={(e) => setLines((ls) => ls.map((x) => (x.key === l.key ? { ...x, qty: e.target.value } : x)))} /> },
              { id: "price", header: "Harga", width: "10rem", align: "right", render: (l, i) => <Input aria-label={`Harga baris ${i + 1}`} inputMode="numeric" className="text-right" value={l.price} onChange={(e) => setLines((ls) => ls.map((x) => (x.key === l.key ? { ...x, price: e.target.value } : x)))} /> },
            ]}
          />
        </FormSection>
        <FormSection title="Info tambahan" description="Pasangan nama dan isi bebas.">
          <KeyValueEditor rows={kv} onChange={setKv} suggestions={["NPWP", "NIB", "Kontak"]} />
        </FormSection>
        <FormSection title="Lampiran">
          <FileDropzone onFile={(f) => notify.success(`Berkas ${f.name} dipilih`)} hint="PDF atau gambar, maks. 5 MB" accept=".pdf,image/*" />
        </FormSection>
      </SectionCard>

      <div className="grid gap-6 xl:grid-cols-2">
        <SectionCard title="Kosong dan memuat" flush>
          <EmptyState title="Belum ada order" description="Order yang dibuat akan muncul di sini." action={<Button size="sm"><Plus /> Buat order pertama</Button>} />
          <TableSkeleton rows={3} cols={4} />
        </SectionCard>
        <SectionCard title="Lapisan">
          <div className="flex flex-col gap-4">
            <Row>
              <Button variant="outline" onClick={() => setModal(true)}>Modal</Button>
              <Button variant="outline" onClick={() => setSheet(true)}>Panel samping</Button>
              <Button variant="outline" onClick={() => setConfirm(true)}>Konfirmasi</Button>
              <Button variant="outline" onClick={() => setReason(true)}>Alasan</Button>
            </Row>
            <CardSkeleton />
          </div>
        </SectionCard>
      </div>

      <Modal
        open={modal}
        onClose={() => setModal(false)}
        title="Ubah alamat"
        description="Dipakai untuk pengiriman berikutnya."
        actions={
          <>
            <Button variant="outline" onClick={() => setModal(false)}>Batal</Button>
            <Button onClick={() => setModal(false)}>Simpan</Button>
          </>
        }
      >
        <FieldGrid>
          <FormField label="Kota">
            <Input defaultValue="Bandung" />
          </FormField>
          <FormField label="Kode pos">
            <Input defaultValue="40115" />
          </FormField>
        </FieldGrid>
      </Modal>
      <FormSheet
        open={sheet}
        onClose={() => setSheet(false)}
        title="Order baru"
        badge={<StatusBadge tone="neutral">Draf</StatusBadge>}
        actions={
          <>
            <Button variant="outline" onClick={() => setSheet(false)}>Batal</Button>
            <Button onClick={() => setSheet(false)}>Simpan</Button>
          </>
        }
      >
        <div className="grid gap-4">
          <FormField label="Pelanggan" required>
            <SearchSelect options={CUSTOMERS} value={customer} onChange={setCustomer} />
          </FormField>
          <FormField label="Tanggal">
            <Input type="date" defaultValue="2026-10-16" />
          </FormField>
          <FormField label="Catatan">
            <Textarea />
          </FormField>
        </div>
      </FormSheet>
      <ConfirmDialog
        open={confirm}
        onOpenChange={setConfirm}
        title="Hapus produk?"
        description="Produk yang dihapus tidak dapat dikembalikan."
        confirmLabel="Hapus"
        onConfirm={() => {
          notify.success("Produk dihapus");
        }}
      />
      <ReasonDialog
        open={reason}
        onClose={() => setReason(false)}
        title="Batalkan surat jalan?"
        description="Stok yang sudah dikurangi dikembalikan."
        confirmLabel="Batalkan surat jalan"
        onSubmit={(r) => {
          setReason(false);
          notify.success(`Dibatalkan: ${r}`);
        }}
      />
    </Page>
  );
}
