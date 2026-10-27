# Rencana — Modul `customers` dan `suppliers`, Go kit `taxid`

> Disusun 27 Oktober 2026. **K1–K9 disetujui sesuai rekomendasi** (27 Oktober 2026);
> **dibangun** hari yang sama (lihat "Hasil pembangunan" di akhir).
> Dasar: diskusi pemisahan `partners` menjadi dua modul (alasan di "Mengapa dua modul"), riset
> internet atas aturan Indonesia dan praktik ERP (sumber di akhir), dan analisis arsitektur
> library: kernel 6, aturan di AGENTS.md, modul `numbering`, `organization`, `settings`.

## Tujuan dan batas

Data induk **mitra dagang** per perusahaan: siapa yang membeli dari perusahaan (`customers`) dan
siapa yang menjual kepadanya (`suppliers`). Keduanya menjadi rujukan `sales`, `purchasing`,
`tax`, `inventory`, dan `accounting` nanti — lewat port, tidak pernah lewat import atau join.

Keduanya:

- menyimpan identitas, identitas pajak (sesuai Coretax), alamat, dan kontak;
- menyimpan **default** dagang (termin, batas kredit, rekening pemasok) — modul transaksi yang
  menegakkannya;
- dinonaktifkan dan bisa ditahan (*on hold*), **tidak pernah dihapus**;
- menawarkan **snapshot** untuk dokumen: dokumen menyalin nama, NPWP, dan alamat saat diposting.

**Bukan** di sini:

| Urusan | Milik |
|---|---|
| Saldo piutang/utang, umur piutang, saling hapus piutang–utang | `accounting` |
| Menolak pesanan karena batas kredit atau status tahan | `sales` / `purchasing` (membaca dari kontrak) |
| Hitungan PPN/PPh, kode transaksi faktur, nomor e-Faktur | `tax` |
| Sertifikat pajak pemasok yang berlaku sampai tanggal tertentu (SKB, Surat Keterangan PP 55/PP 20/2026, DGT/P3B) | `tax` — tabelnya sendiri, `supplier_id` sebagai ID primitif |
| Daftar harga per pelanggan | `items` / `sales` |
| Salesperson dan komisi | `sales` |
| Impor massal dari Excel/CSV | modul impor data nanti (lihat "Di luar 0.1.0") |
| Kode wilayah Kemendagri (provinsi … desa) | kit/modul referensi wilayah nanti |

**Tanpa perubahan kernel.** Semua yang dibutuhkan ada di kernel 6: `d.Tx`, `d.Scope`,
`d.Authz.Units`, `d.Audit` dengan `AuditAccess` (UU PDP).

## Mengapa dua modul (dan bukan `partners`)

Riset ERP (bagian Sumber) menunjukkan dua pola sama-sama mapan: satu catatan pihak (Odoo, Xero,
SAP Business Partner) dan master terpisah (Business Central, ERPNext Customer/Supplier). Untuk
library ini pemisahan dipilih karena:

1. **Proyek memasang yang perlu saja.** POS, klinik, booking hanya butuh pelanggan; aplikasi
   pengadaan hanya pemasok.
2. **Isinya menyimpang seiring waktu.** Pelanggan: batas kredit, grup harga, pemungut PPN.
   Pemasok: rekening bank, potongan PPh, lead time. Semua ERP yang diteliti memisahkan kolom
   khusus peran ini, juga yang satu-catatan (Odoo memakai field terpisah per peran).
3. **Yang hilang bisa ditambal tanpa membongkar.** Peran ganda (satu PT pelanggan sekaligus
   pemasok) ditangani BC dan ERPNext dengan **tautan opsional** antar dua catatan (Contact
   Business Relation, Party Link), bukan penggabungan. Di sini: port opsional nanti (lihat "Di
   luar 0.1.0").
4. **Kode bersama tidak tersalin** — aturan pajak di Go kit `taxid`, komponen generik di
   `ui-forms`.

## Keputusan yang perlu Anda setujui

| # | Keputusan | Rekomendasi | Alternatif | Dasar |
|---|---|---|---|---|
| K1 | Lingkup data | **Per perusahaan** (`"scoped": true`) — pelanggan yang sama di dua PT dicatat dua kali | Satu daftar untuk satu grup (seperti ERPNext/Odoo: catatan global + properti per perusahaan) — butuh modul tidak-scoped dengan isolasi buatan sendiri | BC: data per perusahaan secara default; aturan `scoped-*` menjaga isolasi otomatis |
| K2 | Kode mitra | **Dibuat modulnya sendiri**: kosong → otomatis `PLG-00001` / `PMS-00001` (awalan bisa diubah per perusahaan); boleh diketik manual; unik per perusahaan; boleh ada celah | Ambil dari `numbering` — **ditolak**: README `numbering` menyatakan kode master data bukan urusannya, dan nomor tanpa celah tidak perlu untuk master data | `numbering` › `when_not_to_use`; BC No. Series dengan "manual nos." |
| K3 | NPWP ganda | **Peringatan, bukan larangan**: simpan ditolak 409 dengan daftar mitra ber-NPWP sama, kecuali dikirim `confirm_duplicate: true` | Unik per perusahaan — salah untuk kasus nyata (satu NPWP, beberapa tempat usaha/NITKU yang ditagih terpisah) | Tidak satu pun dari ERPNext, Odoo, BC, Xero mewajibkan NPWP unik; Odoo memberi peringatan |
| K4 | Termin pembayaran di 0.1.0 | **Sederhana di catatan mitra**: `payment_days` (NULL = tunai/COD, 30 = net 30) | Master termin bersama dengan cicilan dan diskon pelunasan cepat (ERPNext/Odoo/BC) — dibuat nanti sebagai modul kecil sendiri yang dihubungkan lewat port opsional, saat `sales` membutuhkannya | Semua ERP memakai master bersama; tetapi belum ada pemakai, dan kolom ini bisa dipetakan ke master nanti |
| K5 | Status | **Aktif/nonaktif** + **tahan** (`on_hold` + alasan) yang dibaca modul transaksi | Tingkat blokir ala BC (pelanggan: kirim / tagih / semua; pemasok: bayar / semua) — ditambahkan saat `sales`/`purchasing` memintanya | BC `Blocked`, ERPNext `on_hold` + `hold_type` |
| K6 | Siapa boleh mengubah | Tambah/ubah data: `*.manage` di unit mana pun (staf cabang sering mendaftarkan pelanggan). **Batas kredit** (`customers.credit`) dan **data sensitif** (`*.sensitive`) harus seluruh-perusahaan (`Units.AllBranches`) | `*.manage` juga seluruh-perusahaan — lebih ketat, staf cabang harus minta ke pusat | ERPNext: batas kredit hanya Credit Manager |
| K7 | Data pribadi (UU PDP) | NIK, paspor, dan nomor rekening **disamarkan** di daftar dan respons biasa (`3201********0001`); nilai utuh lewat endpoint `…/reveal` dengan izin `*.sensitive`, dicatat `AuditAccess` | Tampil utuh bagi pemegang `*.read` | UU 27/2022; PP 33/2026 berlaku 16 Jan 2027; kernel `AuditAccess` dibuat untuk NIK dan rekening |
| K8 | Alamat terstruktur | **Teks bebas berkolom** (jalan, kota/kabupaten, provinsi, kode pos, negara) tanpa kode wilayah | Kode wilayah Kemendagri sekarang — dataset resmi tanpa API, berubah lewat Kepmendagri baru (2022 → 2025, diubah lagi 2025) | Coretax mengisi alamat pembeli NPWP dari database DJP; tidak ditemukan kewajiban alamat terstruktur |
| K9 | Grup pelanggan/pemasok | **Di luar 0.1.0** — ditambahkan sebagai tabel datar per modul (bukan pohon) saat dibutuhkan | Sekarang | ERPNext pohon, Odoo tag datar; belum ada pemakai yang membaca grup |

Dokumen di bawah mengikuti rekomendasi. Bila Anda memilih alternatif, bagian yang terdampak
disebut di tabel "Dampak keputusan".

## Ringkasan

| Bagian | Isi |
|---|---|
| Go kit baru | `taxid` 0.1.0 — validasi dan normalisasi NPWP, NIK, NITKU; penyamaran. Murni (standard library saja) |
| Modul baru | `customers` 0.1.0 dan `suppliers` 0.1.0 — fullstack, `scoped`, `go_kits: ["taxid", "money"]` (`money` untuk batas kredit, hanya `customers`) |
| Tabel | `cust_customers`, `cust_addresses`, `cust_contacts`, `cust_settings` · `supp_suppliers`, `supp_addresses`, `supp_contacts`, `supp_bank_accounts`, `supp_settings` |
| Kontrak | `CustomerClient`, `SupplierClient` |
| Port | Tidak ada di 0.1.0 |
| Event | Tidak ada di 0.1.0 (ditambahkan saat ada pendengar) |
| Kit UI | `ui-forms`: `AddressFields` dan `DigitsInput` (generik, tanpa istilah pajak) — perubahan terpisah, lebih dulu, dengan halaman `ui-gallery` |
| Web | Grup nav "Penjualan" › Pelanggan (order 190), "Pembelian" › Pemasok (order 290) |
| Pengujian | Tes unit; skenario `documents` diperluas (fixture `fixinvoice` memakai pelanggan lewat port) |

## Go kit `taxid`

Aturan format identitas pajak Indonesia, dipakai `customers`, `suppliers`, nanti `tax`, dan bisa
dipakai `settings` (yang kini memvalidasi NPWP sendiri, 15 atau 16 digit). Tanpa tabel, tanpa
kernel, tanpa I/O — memenuhi `gokit-pure`. Nama `taxid` tidak bentrok dengan paket kernel
(`apperror logger pagination request response validator`).

```go
package taxid

// Kind is the type of a party's tax identity, as Coretax names buyer IDs.
type Kind string

const (
	NPWP     Kind = "npwp"     // 16 digits: companies, government, non-resident individuals
	NIK      Kind = "nik"      // 16 digits: resident individuals (NIK is the NPWP, PMK 136/2023)
	Passport Kind = "passport" // foreign individuals
	None     Kind = "none"     // foreign companies, or no tax ID — name and address only
)

// NormalizeNPWP strips punctuation and returns the 16-digit form: 16 digits as given, or a
// 15-digit old NPWP prefixed with "0" (PMK 112/2022). Anything else is ErrFormat.
func NormalizeNPWP(s string) (string, error)

// CheckNIK reports a NIK that is not 16 digits, or whose birth date (day+40 for women) is no
// real date. It cannot prove a NIK exists — only Dukcapil (through Coretax) can.
func CheckNIK(s string) error

// NITKU is the 22-digit place-of-business ID: the head office NPWP16 + a 6-digit sequence
// ("000000" the head office). CheckNITKU also checks the prefix against npwp16 when given.
func CheckNITKU(nitku, npwp16 string) error

// SuspectNPWP reports a check digit that does not match (Luhn over the leading digits, as
// python-stdnum does). DJP has not published the algorithm: a warning, never a refusal.
func SuspectNPWP(npwp16 string) bool

// Mask keeps the first 4 and last 4 characters: "3201********0001".
func Mask(s string) string
```

Bukan di kit: apakah NIK sudah terdaftar sebagai NPWP, status PKP — keduanya hanya bisa dicek di
Coretax/DJP.

## Model data — `customers`

```sql
CREATE TABLE cust_customers (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  code VARCHAR(30) NOT NULL,
  name VARCHAR(150) NOT NULL,             -- nama dagang, tampil di daftar dan pemilih
  legal_name VARCHAR(200) NOT NULL DEFAULT '', -- nama di faktur pajak; '' = name
  party_type VARCHAR(12) NOT NULL,        -- company | individual | government
  is_foreign BOOLEAN NOT NULL DEFAULT FALSE,
  country CHAR(2) NOT NULL DEFAULT 'ID',  -- ISO 3166-1 alpha-2
  tax_id_kind VARCHAR(10) NOT NULL DEFAULT 'none', -- npwp | nik | passport | none (taxid.Kind)
  tax_id VARCHAR(30) NOT NULL DEFAULT '', -- NPWP16/NIK 16 digit, atau nomor paspor
  is_pkp BOOLEAN NOT NULL DEFAULT FALSE,
  vat_collector VARCHAR(12) NOT NULL DEFAULT 'none', -- none | government | other (kode transaksi 02/03)
  email VARCHAR(190) NOT NULL DEFAULT '',
  phone VARCHAR(30) NOT NULL DEFAULT '',
  payment_days INT NULL,                  -- NULL = tunai; K4
  credit_limit DECIMAL(19,2) NULL,        -- NULL = tanpa batas; ditegakkan sales
  on_hold BOOLEAN NOT NULL DEFAULT FALSE,
  hold_reason VARCHAR(255) NOT NULL DEFAULT '',
  notes TEXT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_by BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_by BIGINT NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY cust_customers_code (company_id, code),
  KEY cust_customers_tax (company_id, tax_id),
  KEY cust_customers_name (company_id, is_active, name)
);

-- Alamat tagih, kirim, atau keduanya. NITKU per alamat: satu NPWP punya banyak tempat usaha,
-- masing-masing dengan alamatnya (PER-6/PJ/2024).
CREATE TABLE cust_addresses (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  customer_id BIGINT NOT NULL,
  label VARCHAR(60) NOT NULL,             -- "Kantor pusat", "Gudang Cikarang"
  line VARCHAR(255) NOT NULL,
  city VARCHAR(100) NOT NULL DEFAULT '',  -- kota/kabupaten
  province VARCHAR(100) NOT NULL DEFAULT '',
  postal_code VARCHAR(10) NOT NULL DEFAULT '',
  country CHAR(2) NOT NULL DEFAULT 'ID',
  nitku CHAR(22) NULL,
  for_billing BOOLEAN NOT NULL DEFAULT TRUE,
  for_shipping BOOLEAN NOT NULL DEFAULT TRUE,
  is_default_billing BOOLEAN NOT NULL DEFAULT FALSE,
  is_default_shipping BOOLEAN NOT NULL DEFAULT FALSE,
  is_active BOOLEAN NOT NULL DEFAULT TRUE, -- dokumen lama menunjuk alamat: dinonaktifkan, tidak dihapus
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY cust_addresses_customer (company_id, customer_id)
);

CREATE TABLE cust_contacts (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  customer_id BIGINT NOT NULL,
  name VARCHAR(150) NOT NULL,
  title VARCHAR(100) NOT NULL DEFAULT '', -- jabatan
  email VARCHAR(190) NOT NULL DEFAULT '',
  phone VARCHAR(30) NOT NULL DEFAULT '',
  is_primary BOOLEAN NOT NULL DEFAULT FALSE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY cust_contacts_customer (company_id, customer_id)
);

-- Awalan dan penghitung kode otomatis (K2). Diambil dengan kunci baris di d.Tx; celah boleh.
CREATE TABLE cust_settings (
  company_id BIGINT NOT NULL,
  code_prefix VARCHAR(10) NOT NULL DEFAULT 'PLG-',
  last_number BIGINT NOT NULL DEFAULT 0,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (company_id)
);
```

Kontak orang **dihapus** bila tidak dipakai lagi (tidak ada dokumen yang menunjuknya; aturan
"nonaktifkan" berlaku untuk data yang dirujuk modul lain). Alamat dinonaktifkan, karena
`Snapshot` menerima ID alamat.

## Model data — `suppliers`

Sama bentuknya dengan awalan `supp_`, `code_prefix` default `PMS-`, **tanpa** `vat_collector`
dan `credit_limit`, ditambah rekening bank:

```sql
CREATE TABLE supp_bank_accounts (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  supplier_id BIGINT NOT NULL,
  bank_name VARCHAR(100) NOT NULL,
  bank_code VARCHAR(11) NOT NULL DEFAULT '', -- kode bank 3 digit, atau SWIFT/BIC untuk luar negeri
  account_number VARCHAR(34) NOT NULL,       -- 34: panjang maksimum IBAN
  account_holder VARCHAR(150) NOT NULL,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,   -- pembayaran lama menunjuknya
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY supp_bank_accounts_supplier (company_id, supplier_id)
);
```

Kode bank tidak divalidasi ke daftar resmi (daftar BI tidak tersedia sebagai data terbuka yang
terverifikasi); web menawarkan daftar bank umum sebagai saran. Bila nama pemilik rekening berbeda
dari nama/nama legal pemasok, form memberi peringatan (pencegahan salah transfer dan penipuan
penggantian rekening). Setiap perubahan rekening dicatat sebagai audit `security`.

## Aturan validasi

| Kolom | Aturan |
|---|---|
| `tax_id_kind = npwp` | `taxid.NormalizeNPWP` — 15 digit diubah menjadi 16 dengan awalan "0" (hanya badan; perorangan memakai NIK). `SuspectNPWP` → peringatan di respons, bukan 422 |
| `tax_id_kind = nik` | `taxid.CheckNIK`; hanya untuk `party_type = individual` dan bukan luar negeri |
| `tax_id_kind = passport` | Wajib `is_foreign`; 5–20 karakter huruf/angka |
| `tax_id_kind = none` | Hanya untuk luar negeri, atau dalam negeri tanpa identitas (faktur eceran) — `is_pkp` harus false |
| `is_pkp` | Hanya dengan `tax_id_kind = npwp` atau `nik` |
| `vat_collector = government` | Hanya untuk `party_type = government` |
| `nitku` | `taxid.CheckNITKU` terhadap NPWP mitra (hanya bila `tax_id_kind = npwp` atau `nik`) |
| `code` | Huruf besar, angka, `-`, `/`, `.`; maks. 30; unik per perusahaan (409) |
| `credit_limit` | `money.ParseAmount`, ≥ 0 |
| Alamat default | Paling banyak satu default tagih dan satu default kirim per mitra; menandai yang baru melepas yang lama (dalam satu `d.Tx`) |

Pesan galat dalam bahasa Indonesia, dengan bentuk `validator.FieldError` (422).

## Izin

| Izin | Untuk | Catatan |
|---|---|---|
| `customers.read` | Melihat pelanggan (data sensitif tersamar) | — |
| `customers.manage` | Menambah, mengubah, menonaktifkan, menahan | Di unit mana pun (K6) |
| `customers.credit` | Mengubah batas kredit dan termin | Seluruh-perusahaan |
| `customers.sensitive` | Melihat NIK/paspor utuh | Seluruh-perusahaan; setiap akses → `AuditAccess` |
| `suppliers.read`, `suppliers.manage`, `suppliers.sensitive` | Sama untuk pemasok; `sensitive` juga membuka nomor rekening utuh dan **mengubah rekening** | Mengubah rekening butuh `sensitive` agar tidak bisa diganti tanpa terlihat |

## API

`/api/v1/customers` (dan `/api/v1/suppliers` sejajar):

| Rute | Izin | Isi |
|---|---|---|
| `GET /` `?q=&status=active\|inactive\|on_hold&page=&limit=` | `read` | Daftar berhalaman (`meta`), cari kode/nama/NPWP |
| `GET /{id}` | `read` | Detail dengan alamat dan kontak (tersamar), termasuk yang nonaktif |
| `POST /`, `PUT /{id}` | `manage` (+ `credit` bila batas kredit/termin berubah) | 409 `duplicate_tax_id` dengan daftar mitra kecuali `confirm_duplicate` |
| `POST /{id}/deactivate`, `/activate` | `manage` | — |
| `POST /{id}/hold` `{reason}`, `/release` | `manage` | Alasan wajib |
| `GET /{id}/reveal` | `sensitive` | NIK/paspor (dan rekening, untuk pemasok) utuh; `AuditAccess` |
| `POST /{id}/addresses`, `PUT /{id}/addresses/{aid}`, `POST …/{aid}/deactivate` | `manage` | — |
| `POST /{id}/contacts`, `PUT …/{cid}`, `DELETE …/{cid}` | `manage` | — |
| `GET /settings`, `PUT /settings` | `read` / `manage` seluruh-perusahaan | Awalan kode |
| Pemasok: `POST /{id}/bank-accounts`, `PUT …/{bid}`, `POST …/{bid}/deactivate` | `sensitive` | Audit `security` |

Audit: buat/ubah/nonaktif/tahan → `change`; perubahan batas kredit dan rekening → `security`
dengan nilai sebelum–sesudah; reveal → `access`.

## Kontrak

```go
// CustomerClient is the customers module's public contract.
type CustomerClient interface {
	// Get returns a customer of the company ctx works in, active or not (old documents still
	// render). ErrNotFound for another company's ID.
	Get(ctx context.Context, id int64) (Customer, error)
	// Search finds active customers by code, name, or tax ID — for pickers in other modules.
	Search(ctx context.Context, q string, limit int) ([]Customer, error)
	// Usable is what a document checks before posting, inside its d.Tx: ErrInactive or ErrOnHold.
	// It takes a shared row lock, so a deactivation waits for the posting (and the other way).
	Usable(ctx context.Context, id int64) error
	// Snapshot is what a document copies when posted: legal name, tax ID kind and number
	// (unmasked — it goes on the tax invoice), PKP, VAT collector, and the address addressID
	// (0: the default billing address) as one line with its NITKU.
	Snapshot(ctx context.Context, id, addressID int64) (Snapshot, error)
}

type Customer struct {
	ID          int64
	Code, Name  string
	PartyType   string
	PaymentDays *int
	CreditLimit *money.Amount // nil: no limit
	OnHold      bool
	IsActive    bool
}
```

`SupplierClient` sama, tanpa batas kredit, ditambah `DefaultBankAccount(ctx, id)`. Kontrak
memakai tipe `money.Amount` dari kit — kit bukan modul, jadi tidak melanggar `no-module-imports`;
port konsumen boleh memakai tipe kit yang sama.

**Contoh pemakaian oleh `sales` nanti:** port `sales/ports.Customers` (hanya `Usable`,
`Snapshot`, `Credit`), adapter `adapters/customers_customers.go` di modul `sales`. Penjualan
menyimpan `customer_id` sebagai ID primitif ditambah kolom snapshot di dokumennya.

## Web

- **Pelanggan** (`/customers`), grup "Penjualan", order 190 (dokumen penjualan memakai 100–180);
  **Pemasok** (`/suppliers`), grup "Pembelian", order 290.
- Daftar: `DataTable` dengan cari, filter status (Aktif / Ditahan / Nonaktif), kolom kode, nama,
  NPWP tersamar, kota default, termin; aksi lewat `ActionMenu`.
- Detail/form: `FormSheet` dengan tab Identitas · Alamat · Kontak (· Rekening untuk pemasok) ·
  Dagang. Form NPWP menampilkan peringatan duplikat sebagai `Notice` dengan tombol "Tetap simpan".
- Tombol "Tampilkan" di samping NIK/rekening hanya bila `useCan("…sensitive")`.
- Tahan/nonaktifkan memakai `ReasonDialog` / `ConfirmDialog` (`ui-feedback`).
- Komponen baru di `ui-forms` (perubahan terpisah, lebih dulu): `AddressFields` (jalan, kota,
  provinsi, kode pos, negara — label sebagai prop) dan `DigitsInput` (hanya angka, panjang maks.,
  tampilan berkelompok opsional). Tanpa istilah pajak, memenuhi `domain-agnostic-shared`.

## Pengujian

**Unit** (`kerneltest.Deps`): seluruh aturan validasi; normalisasi 15→16; NIK perempuan
(tanggal+40); default alamat berpindah; kode otomatis dan manual bentrok; batas kredit butuh
`credit`; reveal mencatat `AuditAccess`; `Usable` pada nonaktif/ditahan.

**`taxid`:** tabel kasus format, termasuk contoh NPWP/NIK/NITKU yang valid dan tidak.

**Testbed — perluasan skenario `documents`** (sudah punya admin, organization, dua perusahaan):

- pasang `customers` dan `suppliers`; fixture `fixinvoice` mendapat port `Customers` dengan
  adapter (membuktikan jalur port + `Usable` di dalam `d.Tx`);
- buat pelanggan → kode `PLG-00001`; NPWP 15 digit tersimpan 16 digit;
- NPWP ganda → 409, dengan `confirm_duplicate` → 201;
- pemegang `customers.read` melihat NIK tersamar; `reveal` tanpa `sensitive` → 403, dengan →
  nilai utuh dan satu baris `audit_log` ber-kind `access`;
- staf cabang mengubah batas kredit → 403;
- faktur fixture untuk pelanggan nonaktif/ditahan → 409; `Get` pelanggan nonaktif tetap 200;
- pelanggan perusahaan A tidak terlihat dari perusahaan B (daftar, detail, `Usable`);
- pemasok: ubah rekening tanpa `sensitive` → 403; dengan → audit `security`.

## Risiko yang diverifikasi saat membangun

- **Kunci `Usable` vs nonaktifkan.** Kunci bersama (`SELECT … LOCK IN SHARE MODE`) dari posting
  dan kunci eksklusif dari nonaktifkan; uji dua kontainer agar tidak ada faktur untuk pelanggan
  yang nonaktif di saat yang sama.
- **Penghitung kode.** `last_number` dikunci baris di `d.Tx`; kode manual yang kebetulan sama
  dengan kode otomatis berikutnya → otomatis melompat ke yang kosong.
- **Algoritma check digit NPWP** tidak resmi — hanya peringatan; bila banyak salah positif
  di data nyata, matikan.
- **Pencarian** `LIKE` pada nama per perusahaan cukup untuk ribuan baris; ukur sebelum
  menambahkan FULLTEXT.
- **Status kenaikan tarif PPh 23 tanpa NPWP di 2026** bertentangan antar sumber — tidak
  memengaruhi modul ini (urusan `tax`), tetapi `tax_id_kind` dan `party_type` harus cukup untuk
  memutuskannya.

## Dampak keputusan

| Bila memilih alternatif | Yang berubah |
|---|---|
| K1 grup | Modul tidak `scoped`; semua query dan isolasi dirancang ulang; batas kredit dan termin pindah ke tabel per perusahaan |
| K3 unik | `UNIQUE KEY (company_id, tax_id)` (kecuali kosong); hilangkan `confirm_duplicate` |
| K4 master termin | Modul kecil `payment-terms` lebih dulu; `payment_days` diganti `payment_term_id` + port |
| K5 tingkat blokir | `on_hold` diganti `block` (enum per modul); kontrak `Usable(ctx, id, purpose)` |
| K6 ketat | `manage` diperiksa dengan `Units.AllBranches` |
| K7 utuh | Hapus `reveal` dan izin `sensitive`; audit akses hilang |
| K8 kode wilayah | Kit/modul referensi wilayah lebih dulu; alamat menyimpan kode kelurahan |

## Di luar 0.1.0 (kandidat versi berikutnya)

- Grup pelanggan/pemasok (K9) dan tag.
- Tautan pelanggan ↔ pemasok untuk mitra berperan ganda: port opsional di masing-masing modul
  (late port, karena keduanya saling butuh), untuk tampilan dan saling hapus di `accounting`.
- Impor/ekspor CSV/XLSX (modul impor data generik, lewat `d.Tasks` dan `files`).
- Penggabungan duplikat: memindahkan rujukan seperti BC, bukan menghapus — butuh event agar
  modul lain memindahkan `customer_id`-nya.
- Event `customers.customer.deactivated` dll. saat ada pendengar.
- Kode wilayah Kemendagri sebagai referensi alamat (K8).
- `settings` memakai kit `taxid` untuk NPWP perusahaannya.

## Urutan kerja

1. `ui-forms`: `AddressFields`, `DigitsInput` + halaman `ui-gallery` (perubahan sendiri).
2. Kit `taxid` + tesnya.
3. `customers`: migrasi, sqlc, domain, application, presentation, web, README.
4. `suppliers`: dari `customers`, ditambah rekening bank.
5. Fixture `fixinvoice` memakai port `Customers`; perluas skenario `documents`.
6. Sandbox: lihat setiap halaman.
7. Rilis sesuai BUILDING_MODULES § 7: katalog, STATUS.md (Log, Next: `partners` diganti
   `customers`/`suppliers`), hasil di rencana ini.

## Sumber

**Aturan Indonesia**

- PMK 112/2022 dan PMK 136/2023, NPWP 16 digit/NIK/NITKU — https://pajak.go.id/en/node/96197 ,
  https://www.pajak.go.id/id/artikel/6-tonggak-penerapan-layanan-pajak-berbasis-npwp-16-digit
- Petunjuk penggunaan NPWP dan NITKU (DJP, Sept 2024) — https://www.pajak.go.id/sites/default/files/2024-09/Petunjuk%20Penggunaan%20Nomor%20Pokok%20Wajib%20Pajak%20dan%20Nomor%20Identitas%20Tempat%20Kegiatan%20Usaha%20bagi%20Wajib%20Pajak.pdf
- Pengisian NPWP/NIK pada faktur untuk orang pribadi — https://news.ddtc.co.id/berita/nasional/1821714/ini-ketentuan-pengisian-npwpnik-saat-perekaman-fp-untuk-orang-pribadi
- Keterangan pada faktur pajak (PER-11/PJ/2025) — https://ortax.org/cara-pengisian-keterangan-pada-faktur-pajak
- Faktur untuk subjek pajak luar negeri — https://enforcea.com/Blog/pembuatan-faktur-pajak-kepada-subjek-pajak-luar-negeri-menurut-per-11-pj-2025
- Pedagang eceran tanpa identitas pembeli — https://news.ddtc.co.id/berita/nasional/1817335/pedagang-eceran-dapat-buat-faktur-pajak-tanpa-cantumkan-dua-data-ini
- Galat NITKU pembeli di Coretax — https://news.ddtc.co.id/review/konsultasi-coretax/1808368/muncul-error-nitku-pembeli-tidak-valid-apa-yang-harus-dilakukan
- Kode transaksi faktur di Coretax — https://pajakku.com/artikel/daftar-kode-transaksi-faktur-pajak-terbaru-di-coretax-djp
- Check digit NPWP (tidak resmi) — https://github.com/arthurdejong/python-stdnum/blob/master/stdnum/id/npwp.py
- UU 27/2022 PDP — https://id.wikisource.org/wiki/Undang-Undang_Republik_Indonesia_Nomor_27_Tahun_2022 ;
  PP 33/2026 — https://siplawfirm.id/resources/pp-33-2026-pelindungan-data-pribadi
- Retensi dokumen 10 tahun — https://news.ddtc.co.id/berita/nasional/1805429/alasan-dokumen-dasar-pembukuan-wajib-disimpan-selama-10-tahun
- Kepmendagri kode wilayah 2025 — https://infopublik.id/kategori/nasional-politik-hukum/919675/sukses-launching-kepmendagri-kode-dan-data-wilayah-ditjen-bina-adwil-jawab-harapan-publik
- NIK tidak valid, bukti potong tidak terbit — https://news.ddtc.co.id/berita/nasional/1796474/djp-jika-nik-tidak-valid-bukti-potong-pajak-tidak-dapat-diterbitkan
- PP 20/2026 (revisi PP 55/2022) — https://ikpi.or.id/resmi-pemerintah-perpanjang-fasilitas-pph-final-umkm-lewat-revisi-pp-55/

**Praktik ERP**

- ERPNext Customer / Supplier / Credit Limit / Party Link —
  https://raw.githubusercontent.com/frappe/erpnext/develop/erpnext/selling/doctype/customer/customer.json ,
  https://raw.githubusercontent.com/frappe/erpnext/develop/erpnext/buying/doctype/supplier/supplier.json ,
  https://docs.frappe.io/erpnext/credit-limit , https://docs.frappe.io/erpnext/common-party-accounting
- Business Central Customer / Vendor / Blocked / Payment Terms / Merge —
  https://learn.microsoft.com/en-us/dynamics365/business-central/application/base-application/table/microsoft.sales.customer.customer ,
  https://learn.microsoft.com/en-us/dynamics365/business-central/application/base-application/enum/microsoft.sales.customer.customer-blocked ,
  https://learn.microsoft.com/en-us/dynamics365/business-central/finance-payment-terms ,
  https://learn.microsoft.com/en-us/dynamics365/business-central/sales-how-merge-duplicate-records ,
  https://learn.microsoft.com/en-us/dynamics365/business-central/marketing-create-contact-companies
- Odoo res.partner, payment term, merge —
  https://raw.githubusercontent.com/odoo/odoo/18.0/odoo/addons/base/models/res_partner.py ,
  https://raw.githubusercontent.com/odoo/odoo/18.0/addons/account/models/partner.py ,
  https://raw.githubusercontent.com/odoo/odoo/18.0/odoo/addons/base/wizard/base_partner_merge.py
- SAP Business Partner / CVI — https://blogs.sap.com/2016/04/29/simplification-item-business-partner-approach-customer-vendor-integration-cvi

**Belum terkonfirmasi** (tidak menjadi dasar keputusan keras): algoritma check digit NPWP resmi;
batas akhir penerimaan NPWP 15 digit; status NIK sebagai data pribadi spesifik; daftar kode bank
resmi BI; berlakunya kenaikan tarif PPh 23 tanpa NPWP di 2026.

## Hasil pembangunan

Dibangun 27 Oktober 2026 di branch `customers-suppliers`, sesuai K1–K9.

| Rilis | Isi |
|---|---|
| `ui-forms` 0.2.0, `ui-gallery` 0.1.1 | `DigitsInput`, `AddressFields` (generik), dengan contoh di galeri |
| `taxid` 0.1.0 | `NormalizeNPWP`, `CheckNIK`, `CheckNITKU`, `CheckPassport`, `SuspectNPWP`, `Mask` |
| `customers` 0.1.0 | Sesuai rancangan: tabel `cust_*`, `CustomerClient`, halaman daftar dan detail |
| `suppliers` 0.1.0 | Sesuai rancangan: tabel `supp_*` dengan rekening bank, `SupplierClient` dengan `DefaultBankAccount` |
| fixture `fixinvoice` 0.2.0 | Port opsional `Customers` (adapter ke `customers`): `Usable` dan `Snapshot` di dalam `d.Tx` posting |

**Penyesuaian saat dibangun.**

- **Nama metode validasi domain `Clean`, bukan `Prepare`.** Aturan `tx-aware` membaca `.Prepare(` sebagai
  pernyataan SQL.
- **`suppliers` tanpa izin `credit`.** Termin pemasok diubah dengan `suppliers.manage`, dan
  perubahannya dicatat sebagai audit `change` dengan nilai sebelum dan sesudah. Batas kredit
  hanya milik pelanggan, sesuai plan.
- **Reveal pemasok mencakup rekening.** Satu pembacaan yang diaudit menampilkan identitas pajak
  dan semua nomor rekening utuh. Mengubah rekening dimulai dari nomor utuhnya.
- **`DefaultBankAccount` mencatat audit `access`.** Rekening utuh yang dibaca modul pembayaran ikut
  tercatat.
- **Endpoint `GET /lookup?q=`.** Untuk pemilih di halaman modul lain.

**Bukti.**

- Tes unit di `customers` dan `suppliers` (`application`) dan `taxid`: lolos.
- Skenario testbed `documents` diperluas: kode otomatis, NPWP 15→16, NPWP ganda (409 lalu
  konfirmasi), NIK dan NITKU ditolak dengan pesan per kolom, NIK tersamar dan reveal ber-audit,
  staf tanpa `credit`/`sensitive` ditolak, faktur fixture untuk pelanggan ditahan atau nonaktif
  ditolak (409) lewat port, rekening dijaga dan diaudit tanpa nomor utuh, isolasi perusahaan.
- Testbed lengkap: `All testbed checks passed: 420 checks.`
- Setiap halaman dilihat di sandbox: daftar, detail, form, reveal NIK, rekening pemasok.

**Batasan yang diterima** (juga di STATUS.md › Known limits):
- data per perusahaan;
- check digit NPWP tidak resmi;
- alamat berupa teks bebas.
