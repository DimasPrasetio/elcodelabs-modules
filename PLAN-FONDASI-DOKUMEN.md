# Rencana — Fondasi dokumen bisnis: kit `money` dan `docstate`, modul `numbering` dan `period`

> Disusun 27 Oktober 2026. **Dibangun 27 Oktober 2026** (lihat "Hasil pembangunan"); belum di-commit. Dasar: laporan riset
> `Cakupan kernel 6 platform modular` (repo skill; catatan `frappe_odoo.md`, `go_mysql_patterns.md`,
> `erp_hrmis_wms_indonesia.md`), kernel 6 (`d.Tx`, `d.Events`, lingkup perusahaan/cabang), modul
> `organization` 0.1.0.

## Tujuan dan batas

Setiap modul transaksi berikutnya — `sales`, `purchasing`, `inventory`, penggajian — butuh empat hal
yang sama. Bila tiap modul membuatnya sendiri, hasilnya pasti berbeda-beda:

| Butuh | Jawaban | Jenis | Mengapa jenis itu |
|---|---|---|---|
| Angka uang dan kuantitas yang tidak pernah salah satu sen | `money` | **Go kit** | Fungsi murni: tanpa tabel, tanpa wiring |
| Siklus dokumen draft → posted → batal → revisi | `docstate` | **Go kit** | Aturan transisi murni; status disimpan di tabel modul pemilik dokumen |
| Nomor dokumen per perusahaan/cabang/periode, tanpa lompatan | `numbering` | **Modul** (*scoped*) | Punya tabel penghitung dan halaman pengaturan |
| Tutup buku: tanggal yang sudah dikunci tidak bisa diposting atau dibatalkan | `period` | **Modul** (*scoped*) | Punya tabel tanggal kunci dan halaman pengaturan |

**Bukan** di sini: menghitung PPN dan faktur pajak Coretax (modul `tax` nanti, memakai `money`),
jurnal dan buku besar (modul `accounting`), persetujuan bertingkat (`approvals`), multi-mata-uang
(lihat K4). **Tanpa perubahan kernel**: semuanya dibangun di atas `d.Tx`, `d.Events`, `d.Scope`,
`d.Authz`, dan `d.Audit` kernel 6.

## Keputusan yang perlu Anda setujui

| # | Keputusan | Rekomendasi | Alternatif |
|---|---|---|---|
| K1 | Pustaka desimal di bawah `money` | **`github.com/govalues/decimal`** (MIT): tidak pernah panic (galat saat overflow), presisi 19 digit, sudah `sql.Scanner`/`driver.Valuer`/JSON-sebagai-string, cepat. Dibungkus, jadi bisa diganti tanpa mengubah modul | `shopspring/decimal` (paling populer, presisi tak terbatas, tetapi bisa panic dan lebih lambat); tipe fixed-point buatan sendiri (tanpa dependensi, tetapi perkalian dan pembulatannya harus ditulis dan diuji sendiri) |
| K2 | Penomoran | **Selalu tanpa lompatan (*gapless*)**: penghitung dikunci baris (`FOR UPDATE`) di transaksi pemanggil, jadi rollback mengembalikan nomornya. Nomor diberikan saat **posting**, bukan saat draft dibuat | Boleh memilih per seri: cepat (bisa berlompat) atau tanpa lompatan |
| K3 | Model tutup buku | **Tanggal kunci per perusahaan dan per area** (`*` = semua, lalu `sales`, `purchasing`, `inventory`, `payroll`, …). Satu tanggal "terkunci sampai dengan", seperti *lock date* di Odoo | Baris periode per bulan dengan status buka/tutup (ERPNext *Accounting Period*) — lebih banyak baris dan lebih banyak keadaan, untuk manfaat yang sama |
| K4 | Mata uang | **Rupiah saja di versi pertama**: `money.Amount` tanpa kode mata uang. Pemicu menambah mata uang: modul pertama yang benar-benar butuh valas (impor/ekspor) | Membawa kode mata uang dari awal di setiap nilai |
| K5 | Revisi dokumen yang sudah diposting | **Batal lalu revisi**: dokumen posted tidak pernah diubah; ia dibatalkan, lalu revisinya adalah dokumen draft baru yang menunjuk ke yang dibatalkan (`amended_from`) dan diberi nomor `<nomor asal>-1`, `-2`, … | Mengubah dokumen posted di tempat dengan riwayat versi — melanggar prinsip *immutable* buku |
| K6 | Kunci permanen (*hard lock*) | **Belum**: tanggal kunci boleh dimundurkan oleh pemegang `period.unlock` dengan alasan, tercatat di audit `security` | Kunci permanen yang tidak bisa dibuka siapa pun (Odoo *Hard Lock*); pemicu: auditor atau regulasi yang memintanya |

Dokumen ini ditulis mengikuti rekomendasi. Bila Anda memilih alternatif, bagian yang terdampak
tercantum di tabel "Dampak keputusan" di akhir.

## Kit `money` 0.1.0

Dipasang ke `apps/api/internal/shared/money`. Modul yang memakainya menulis `"go_kits": ["money"]`.

### Tipe

| Tipe | Untuk | Kolom MySQL | Skala |
|---|---|---|---|
| `money.Amount` | Nilai uang: harga total, DPP, PPN, saldo | `DECIMAL(19,2)` | 2 (sen) |
| `money.Qty` | Kuantitas: 2,5 kg, 0,125 m³ | `DECIMAL(19,4)` | 4 |
| `money.Rate` | Tarif dan pengali: 0,12 (PPN 12 %), 11/12, kurs | `DECIMAL(19,10)` | hingga 10 |

Ketiga tipe tidak bisa dicampur sembarangan: `Amount.Add(Amount)`, tetapi tidak
`Amount.Add(Qty)`. Kompilator yang menolak salah ketik "kuantitas ditambah rupiah".

### Operasi

- `Add`, `Sub`, `Neg`, `Abs`, `Cmp`, `IsZero`, `Sum(...)`.
- `Amount.MulQty(Qty, Rounding)`, `Amount.MulRate(Rate, Rounding)` → `Amount`, dibulatkan sekali di
  akhir dengan kebijakan yang **dipilih pemanggil**.
- `Amount.MulFrac(num, den, Rounding)` — perkalian pecahan tepat. Contohnya DPP nilai lain 11/12
  menurut PMK 131/2024: 11/12 × Rp10.000.000 = Rp9.166.666,67 sebelum dibulatkan.
- `Allocate(total Amount, weights []Qty|[]Amount) []Amount` — membagi total ke baris (diskon faktur,
  biaya kirim) dengan metode sisa terbesar, sehingga **jumlah bagiannya selalu sama dengan totalnya**.
- Pembulatan: `HalfUp` (pembulatan komersial: ≥ 0,5 ke atas — default), `HalfEven`, `Up`, `Down`,
  ke skala mana pun (`Round(0)` = rupiah penuh). `govalues` hanya punya *half-even*; *half-up*
  dibuat di kit dan diuji dengan tabel kasus batas (0,5; −0,5; 0,49999…).
- Teks: `ParseAmount("1234567.89")` dan `String()` dengan titik desimal kanonik untuk API dan
  database; `Format(Amount)` → "Rp1.234.567,89" (id-ID, tanpa ",00" bila bulat — opsi).
- Database: `Scan`/`Value`. **sqlc** memetakan `DECIMAL` MySQL ke `string`, jadi repository
  memakai `money.ParseAmount(row.Total)` saat membaca dan `amount.String()` saat menulis. Ini
  diverifikasi saat membangun; bila sqlc memberi tipe lain, kit menyediakan adaptornya.
- JSON: selalu **string** (`"1234567.89"`), karena angka JSON di JavaScript adalah float. Web
  memakai `formatMoney` milik `ui-core`, yang menerima string.

**Bukan di kit:** aturan pajak (kapan 11/12, kapan 12 % penuh untuk barang mewah, pembulatan per
faktur atau per baris). Itu kebijakan modul `tax` nanti, yang **memakai** operasi di atas.
Penelusuran untuk rancangan ini tidak menemukan aturan pembulatan PPN yang eksplisit; modul `tax`
harus memastikannya dari peraturan, bukan dari kit.

### Aturan library baru (usul)

`exact-numbers`: migrasi modul tidak boleh membuat kolom `FLOAT`, `DOUBLE`, atau `REAL`. Uang dan
kuantitas memakai `DECIMAL`. Kolom yang memang butuh float (koordinat peta) ditandai dengan baris
`-- float: <alasan>`. Diperiksa `catalog.mjs`, sejalan dengan aturan lain.

## Kit `docstate` 0.1.0

Dipasang ke `apps/api/internal/shared/docstate`. Tanpa tabel: status disimpan di kolom `status`
tabel milik modul dokumen, dan kit menetapkan aturannya.

### Status dan transisi

```
draft ──post──▶ posted ──cancel(alasan)──▶ cancelled ──amend──▶ (dokumen baru) draft
  │
  └─delete──▶ (hilang: draft boleh dihapus selama tak ada yang menunjuknya)
```

| Dari | Aksi | Ke | Syarat yang dicek kit |
|---|---|---|---|
| `draft` | `edit` | `draft` | — |
| `draft` | `post` | `posted` | — (pemanggil yang mengecek periode dan nomor) |
| `draft` | `delete` | — | — |
| `posted` | `cancel` | `cancelled` | Alasan wajib (≥ 5 karakter) |
| `cancelled` | `amend` | dokumen baru `draft` | Hanya dari dokumen yang dibatalkan |
| lainnya | — | — | `docstate.ErrTransition` dengan pesan Indonesia ("Dokumen yang sudah diposting tidak dapat diubah; batalkan lalu buat revisinya"); modul memetakannya ke 409 |

### Isi kit

- `type Status string`, konstanta `Draft`, `Posted`, `Cancelled`.
- `Check(from Status, action Action) error` dan `Next(from, action) (Status, error)`.
- `Fields`: struktur yang ditanam modul di domainnya — `Status`, `PostedAt`, `PostedBy`, `CancelledAt`,
  `CancelledBy`, `CancelReason`, `AmendedFrom`, `Revision` — beserta kolom SQL yang dianjurkan.
- `AmendedNumber(base string, revision int) string` → `INV/2026/10/0007-1`.
- `Guard`: pola SQL yang dianjurkan untuk transisi tanpa balapan:
  `UPDATE … SET status = 'posted', … WHERE id = ? AND company_id = ? AND status = 'draft'` → 0 baris
  berarti `docstate.ErrStale` ("Dokumen sudah diubah orang lain — muat ulang"; modul memetakannya
  ke 409).

Kit hanya mengimpor pustaka standar (aturan `gokit-pure`): galatnya galat Go biasa yang bisa dicek
dengan `errors.Is`, dan modul yang mengubahnya menjadi respons HTTP lewat `shared/apperror`.

### Resep posting (README kit, dan contoh di testbed)

Satu `d.Tx` memuat semuanya, sehingga semuanya terjadi atau tidak sama sekali:

1. Kunci baris dokumen (`SELECT … FOR UPDATE`), lalu `docstate.Check(status, Post)`.
2. `period.EnsureOpen(ctx, "sales", tanggalDokumen)`.
3. `numbering.Next(ctx, "sales.invoice", tanggalDokumen)`.
4. `UPDATE` status dengan *guard*, tulis nomor, `PostedAt`, dan `PostedBy`.
5. `d.Events.Publish(ctx, "sales.invoice.posted", 1, …)` — misalnya `inventory` mengurangi stok.

Batal: `EnsureOpen` untuk **tanggal dokumen** (buku yang sudah ditutup tidak berubah). Pembalikan
efeknya (jurnal balik, stok kembali) adalah urusan modul lewat event `…cancelled`.

## Modul `numbering` 0.1.0

Fullstack, *scoped* (setiap tabel ber-`company_id`). Menawarkan `NumberingClient`.

### Tabel

```sql
-- Format per jenis dokumen per perusahaan; tanpa baris → format bawaan yang dideklarasikan modul.
CREATE TABLE num_formats (
  company_id BIGINT NOT NULL,
  doc_type VARCHAR(100) NOT NULL,      -- "sales.invoice"
  pattern VARCHAR(100) NOT NULL,       -- "INV/{BRANCH}/{YYYY}/{MM}/{SEQ:4}"
  reset VARCHAR(10) NOT NULL,          -- never | yearly | monthly | daily
  per_branch BOOLEAN NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (company_id, doc_type)
);
-- Penghitung: satu baris per seri (perusahaan, jenis, cabang, periode).
CREATE TABLE num_counters (
  company_id BIGINT NOT NULL,
  doc_type VARCHAR(100) NOT NULL,
  branch_id BIGINT NOT NULL DEFAULT 0,
  period_key VARCHAR(10) NOT NULL,     -- "" | "2026" | "2026-10" | "2026-10-27"
  last_number BIGINT NOT NULL,          -- "last_value" kata kunci parser sqlc
  PRIMARY KEY (company_id, doc_type, branch_id, period_key)
);
```

### Kontrak

```go
type NumberingClient interface {
	// Declare registers a document type and its default format — in the consumer's New.
	Declare(docType, label string, def Format)
	// Next allocates the next number of docType for date, in the scope ctx works in (company,
	// branch). It must run inside d.Tx: the counter row stays locked until that transaction
	// ends, and a rollback gives the number back — no gaps.
	Next(ctx context.Context, docType string, date kernel.Date) (string, error)
	// Preview is what Next would return now, without taking it (for forms).
	Preview(ctx context.Context, docType string, date kernel.Date) (string, error)
}
```

### Perilaku

- **Tanpa lompatan (K2):** `INSERT … ON DUPLICATE KEY UPDATE last_number = last_number + 1`, lalu
  `SELECT … FOR UPDATE`, di transaksi pemanggil. Dua kasir yang memposting bersamaan berbaris di
  baris penghitung yang sama — milidetik untuk UKM. Bila terjadi *deadlock*, `d.Tx` mengulang.
  `Next` di luar `d.Tx` menghasilkan galat, bukan nomor yang bisa hilang.
- **Pola:** `{YYYY}` `{YY}` `{MM}` `{DD}` `{SEQ:n}` (wajib ada, diisi nol sampai n digit), `{BRANCH}`
  (kode cabang dari port opsional `Branches` → `organization`; tanpa port: ID cabang), `{COMPANY}`,
  dan teks biasa. Validasi menolak format yang bisa menghasilkan nomor kembar, misalnya
  `reset = yearly` tanpa `{YYYY}`/`{YY}`, atau `per_branch` tanpa `{BRANCH}`.
- **Tanggal:** periode reset dihitung dari tanggal dokumen dalam zona lingkup (`kernel.Date`),
  bukan dari jam server.
- **Pindah dari sistem lama:** "Set nomor berikutnya" (izin `numbering.manage`, hanya **naik**, dengan
  alasan; audit `security`), supaya nomor lanjutan tidak bertabrakan dengan arsip lama.
- **Halaman** Pengaturan › Penomoran: daftar jenis dokumen yang dideklarasikan modul terpasang,
  formatnya, pratinjau nomor berikutnya per cabang, dan ubah format.
- **Izin:** `numbering.read`, `numbering.manage`.
- **Bukan nomor faktur pajak:** NSFP/nomor faktur e-Faktur diberikan Coretax dan disimpan modul
  `tax`; `numbering` hanya untuk nomor dokumen internal.

## Modul `period` 0.1.0

Fullstack, *scoped*. Menawarkan `PeriodClient`.

### Tabel

```sql
CREATE TABLE period_locks (
  company_id BIGINT NOT NULL,
  area VARCHAR(50) NOT NULL,           -- "*" semua, atau "sales", "purchasing", "payroll", …
  locked_through DATE NOT NULL,        -- tanggal ini dan sebelumnya tertutup
  updated_by BIGINT NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (company_id, area)
);
CREATE TABLE period_settings (
  company_id BIGINT NOT NULL,
  fiscal_year_start_month TINYINT NOT NULL DEFAULT 1, -- 1 = Januari
  PRIMARY KEY (company_id)
);
```

### Kontrak

```go
type PeriodClient interface {
	// Declare registers an area that can be locked on its own — in the consumer's New.
	Declare(area, label string)
	// EnsureOpen refuses (ErrClosed → 409) a date on or before the area's lock or the
	// company-wide one ("*"). Inside d.Tx it reads the lock rows FOR SHARE, so a lock being
	// moved at the same moment waits for the posting, or the posting for it — never both.
	EnsureOpen(ctx context.Context, area string, date kernel.Date) error
	// FiscalYear is the fiscal year date falls in (start and end dates) for the company.
	FiscalYear(ctx context.Context, date kernel.Date) (start, end kernel.Date, err error)
}
```

### Perilaku

- **Maju** (menutup lebih banyak): izin `period.manage`.
- **Mundur** (membuka lagi): izin `period.unlock`, alasan wajib, audit `security` (K6). Kunci
  permanen belum ada.
- **Tanggal kunci area tidak boleh lebih awal dari kunci `*`:** penutupan semua area selalu menang.
- **Tahun buku:** bulan awal per perusahaan, untuk laporan dan untuk retensi 10 tahun "sejak akhir
  tahun buku" (UU KUP Pasal 28 ayat 11).
- **Halaman** Pengaturan › Tutup buku: tanggal kunci per area, bulan awal tahun buku, riwayat
  perubahan (dari audit).
- **Izin:** `period.read`, `period.manage`, `period.unlock`.

## Cara modul lain memakainya (tanpa saling import)

```
sales (konsumen)                      composition root                  penyedia
  ports.Numbering  ──adapter──▶  salesNumbering{numberingModule.Client()}   numbering
  ports.Periods    ──adapter──▶  salesPeriods{periodModule.Client()}        period
  internal/shared/money, internal/shared/docstate  (Go kit: import langsung, dideklarasikan di go_kits)
```

- Port ke `numbering` dan `period` **wajib** bagi modul dokumen: tanpa nomor dan tanpa tutup buku,
  dokumen bisnis tidak benar. Adapter siap pakai dikirim bersama modul konsumennya.
- Kit tidak butuh port: ia kode bersama, bukan modul.

## Pengujian

**Unit:**
- `money`: tabel pembulatan (*half-up*/*half-even*, nilai negatif, batas 0,5); `Allocate` selalu
  berjumlah total (properti diuji acak); `MulFrac` 11/12; *overflow* menghasilkan galat; JSON dan
  `Scan`/`Value` pulang-pergi; format id-ID.
- `docstate`: seluruh tabel transisi; `AmendedNumber`.
- `numbering`: validasi pola; periode reset per zona; pratinjau tanpa mengambil nomor.
- `period`: aturan maju/mundur dan izin; area vs `*`; tahun buku dengan bulan awal Juli.

**Testbed — skenario baru `documents`** (MySQL sungguhan), dengan modul fixture `fixinvoice` yang
memakai keempatnya:

| Pemeriksaan | Cara |
|---|---|
| Tanpa lompatan | Posting yang di-rollback (stok kurang) mengembalikan nomornya; posting berikutnya memakainya |
| Konkurensi | 50 posting bersamaan dari dua kontainer API → nomor 1…50 tanpa kembar, tanpa lompatan |
| Reset | Ganti bulan → `…/11/0001`; per cabang → seri sendiri; pola kembar ditolak |
| Tutup buku | Posting dan batal bertanggal di periode tertutup → 409; buka kembali tanpa `period.unlock` → 403, dengan izin dan alasan → bisa |
| Balapan kunci | Kunci digeser saat posting berjalan → salah satu menunggu, hasilnya konsisten |
| Siklus | Ubah dokumen posted → 409; batal tanpa alasan → 422; revisi → draft `…-1` yang menunjuk asalnya |
| Uang | Nilai DECIMAL pulang-pergi tanpa selisih; pembagian diskon ke baris berjumlah tepat |
| Lingkup | Nomor dan kunci perusahaan A tidak memengaruhi perusahaan B |

## Risiko dan yang harus diverifikasi saat membangun

| Hal | Cara |
|---|---|
| Tipe Go sqlc untuk `DECIMAL` di MySQL, dan bentuk nilai dari driver (`[]byte`) | Dicoba di sandbox sebelum API kit dikunci |
| Versi `govalues/decimal` terbaru yang stabil | Dipin di `go_dependencies` dengan versi yang diuji |
| Penghitung tanpa lompatan menjadi titik antre saat posting sangat ramai | Diukur di testbed (50 bersamaan); bila perlu, seri per cabang memecah antrean |
| Konsumen lupa memanggil `EnsureOpen` atau `Next` di dalam `d.Tx` | `Next` menolak di luar transaksi; resep dan fixture di README; tinjauan |

## Urutan pengerjaan

1. Kit `money` (beserta aturan `exact-numbers`).
2. Kit `docstate`.
3. Modul `period`.
4. Modul `numbering` (port opsional `Branches` → `organization`).
5. Fixture `fixinvoice` dan skenario testbed `documents`; testbed lengkap.
6. Tinjauan independen (kebenaran angka, balapan nomor dan kunci, keamanan izin buka kunci),
   perbaiki, ulangi sampai lulus.
7. Commit setelah Anda setujui.

## Hasil pembangunan

### Yang dibangun

| Bagian | Isi | Uji |
|---|---|---|
| Kit `money` 0.1.0 | `Amount`/`Qty`/`Rate`, pembulatan sekali di akhir (empat cara), `Allocate`, format id-ID, SQL dan JSON | 91 % cakupan; uji acak pembagian (2.000 kasus) dan tabel batas setengah |
| Kit `docstate` 0.1.0 | Status, aksi, tabel transisi, `CancelReason`, `Guard`, `Fields`, `Amendment` | 97 % cakupan |
| Modul `period` 0.1.0 | Kunci per perusahaan dan area, buka kembali dengan izin dan alasan, tahun buku, halaman Tutup buku | Uji unit layanan dan domain |
| Modul `numbering` 0.1.0 | Format per perusahaan, penghitung tanpa lompatan, catatan nomor yang pernah diberikan, pratinjau, nomor berikutnya, halaman Penomoran | Uji unit layanan dan domain |
| Aturan `exact-numbers` | `catalog.mjs` menolak kolom FLOAT/DOUBLE/REAL tanpa `-- float: <alasan>`; AGENTS.md | Diuji menolak dan menerima di testbed |
| Testbed `documents` | Fixture `fixinvoice` memakai keempatnya; MySQL sungguhan, dua kontainer | Semua pemeriksaan lulus (lihat bukti) |

### Penyesuaian dari rancangan

| Hal | Rancangan | Dibangun | Alasan |
|---|---|---|---|
| Kolom penghitung | `last_value` | `last_number` | `last_value` kata kunci di parser MySQL sqlc |
| Tabel tambahan | — | `num_issued (company_id, doc_type, number)` unik | Tinjauan: kode cabang yang diganti/dipakai ulang bisa mencetak nomor lama. Nomor yang pernah diberikan dilewati, jadi **tidak pernah** tercetak dua kali |
| Port opsional | `Branches` | `Units` (kode perusahaan, kode cabang, daftar cabang) | `{COMPANY}` juga butuh kode |
| Ubah format | Bebas | Cara mulai ulang dan per-cabang **terkunci** setelah nomor pertama dipakai (409); pola tetap boleh diubah | Tinjauan: bulanan → tahunan mencetak ulang `INV/2026/10/0001` |
| Validasi pola | Tahun/bulan/hari dan `{BRANCH}` | + pemisah kode bukan `-`; pola tidak diakhiri kode; nomor ≤ 90 karakter | Tinjauan: kode boleh memuat `-`, revisi `…-1`, dan `VARCHAR(100)` |
| Nomor berikutnya | Hanya naik | + paling besar 999 999 999; urutan berhenti di 999 999 999 999 dengan 409 | Tinjauan: BIGINT bisa dibuat penuh dan seri mati |
| Serialisasi kunci | `FOR SHARE` pada baris kunci | `INSERT IGNORE` baris `period_settings` (shared) per posting; ubah kunci memegangnya exclusive | Baris kunci yang belum ada tidak bisa dikunci di READ COMMITTED; baris pengaturan selalu ada |
| Tanggal kunci | — | Harus sebelum hari ini; kunci area tidak boleh lebih awal dari `*` | Mencegah salah klik menutup hari berjalan |
| Riwayat di halaman Tutup buku | Daftar dari audit | Catatan "lihat log audit" | `period` tidak boleh membaca modul `audit-log` |
| Resep posting | Nomor sebelum stok | `EnsureOpen` paling awal, **nomor paling akhir** | Tinjauan: kunci penghitung ditahan sampai commit; `EnsureOpen` setelah kunci baris lain bisa deadlock saat kunci sedang digeser |
| Galat 422 | — | `[]validator.FieldError` | Terlihat saat cek visual: peta tidak terbaca web |
| JSON uang yang salah | 422 | 400 "Kolom lines.price berisi tipe data yang salah" | `request.DecodeJSON` kernel memetakan `json.UnmarshalTypeError`; pesan per aturan butuh field string |
| Hak tombol di halaman | `useCan` | Server mengirim `can_close`/`can_reopen`, `manage_formats`, `manageable` per seri | `useCan` tidak berarti "di seluruh perusahaan" |
| Ubah tahun buku | Audit biasa | Audit `security` | Menggeser cakupan penutupan per tahun buku |

### Tinjauan independen

Tiga peninjau terpisah (kebenaran angka; balapan nomor dan kunci, dengan MySQL; keamanan izin
dan lingkup). Semua temuan terkonfirmasi diperbaiki:

| Temuan | Perbaikan |
|---|---|
| `Parse` membulatkan diam-diam input > 19 digit | Skala dan digit diperiksa pada teks sebelum pustaka desimal |
| `Round(0)` dekat batas menghasilkan Amount di luar DECIMAL(19,2) | Hasil dibawa kembali ke skala 2 dengan cek overflow |
| `Allocate` kuadratik (20.000 baris ≈ 10 dtk) | Urut stabil O(n log n) |
| Spasi, `Rounding` tak dikenal | Ditolak (`ErrSyntax`, `ErrRounding`) |
| Ganti reset/per-cabang mengulang nomor | Dikunci setelah nomor pertama + `num_issued` |
| Kode cabang berganti/sama dengan kode perusahaan | `num_issued`: nomor yang pernah dicetak dilewati (tolakan kode sama dicabut di putaran 2: menyebabkan 500, tidak lagi perlu) |
| Pemisah `-` dan akhiran revisi | Aturan pola baru |
| Nomor berikutnya tanpa batas; panjang nomor tanpa batas | Naikkan ≤ 999 999 999; urutan ≤ 999 999 999 999; nomor ≤ 90 karakter; galatnya 409, bukan 500 |
| Tombol tampil untuk peran satu cabang | Hak dari server |
| Pembaca satu cabang melihat seri cabang lain | Seri disaring dengan `Units(numbering.read)` |
| RaiseNext membaca format di luar transaksi | Dibaca ulang di dalam transaksi |
| Putaran 2: `num_issued` peka huruf besar-kecil? | Kolom `utf8mb4_bin` |
| Putaran 2: pratinjau bisa berbeda dari `Next` | Pratinjau melewati nomor yang pernah diberikan |
| Putaran 2: tombol "Ubah" area yatim tanpa `period.unlock` | Disembunyikan |
| Putaran 2: nomor revisi tidak ada di `num_issued`; pola baru bisa mencetak `X-1` | `Next` melewati nomor berbentuk `X-k` bila `X` pernah diberikan |
| Putaran 2: RaiseNext dan ubah format bisa bersilang | RaiseNext membaca format `FOR SHARE` |

**Diterima, dicatat:** tiga posting yang mencetak string sama pada saat bersamaan (kode
dipakai ulang) bisa deadlock — `d.Tx` mengulang. `num_issued` per jenis dokumen: dua jenis yang
ditulis ke satu tabel konsumen dengan pola sama tidak saling dicek (konsumen memakai satu jenis
per tabel). Antrean kunci `period_settings` dapat mengubah urutan kunci yang cocok
menjadi deadlock bila modul memanggil `EnsureOpen` setelah kunci baris lain — `d.Tx` mengulang;
resep dan README menyuruh `EnsureOpen` paling awal. Kernel `d.Audit` hanya mencatat log bila
penulisan audit gagal, sehingga perubahan bisa tersimpan tanpa catatannya — urusan kernel
(usul permintaan kernel terpisah). Data halaman lama terlihat sesaat setelah ganti perusahaan
(`useResource`), server tetap memeriksa ulang.

## Dampak keputusan

| Bila memilih alternatif | Yang berubah |
|---|---|
| K1 shopspring | Kit menangkap panic di setiap operasi; lebih lambat. API kit tetap |
| K1 buatan sendiri | Perkalian 128-bit dan pembulatan ditulis dan diuji sendiri; tanpa dependensi |
| K2 boleh pilih per seri | Kolom `mode` di `num_formats`; seri cepat memakai `AUTO_INCREMENT`/penghitung tanpa kunci, dan dokumentasinya menjelaskan lompatan |
| K3 periode bulanan | Tabel `period_periods (company, area, year, month, status)`; `EnsureOpen` mencari baris bulan itu; halaman berupa kalender |
| K4 valas sekarang | `money.Money{Amount, Currency}` di setiap nilai, kurs (`Rate`) dan konversi; kolom mata uang di setiap tabel konsumen |
| K5 ubah di tempat | `docstate` punya transisi `posted → posted (revisi)` dan tabel versi di modul pemilik dokumen |
| K6 kunci permanen | Kolom `hard_locked_through` yang hanya bisa maju |

## Sumber

- Frappe `naming.py` (penghitung `tabSeries` dengan `SELECT … FOR UPDATE` di transaksi pemanggil;
  amandemen dengan akhiran `-1`) dan `docstatus` 0/1/2: ringkasan di catatan riset `frappe_odoo.md`.
- Odoo 18 *lock dates* (kunci penjualan, pembelian, pajak, "lock everything", *hard lock* permanen):
  [Cybrosys — Odoo 18 Accounting: Lock Dates](https://www.cybrosys.com/odoo/odoo-books/v18/accounting/lock-dates/),
  [Odoo documentation PR](https://github.com/odoo/documentation/pull/15305/files),
  [forum Odoo](https://odoo.com/ko_KR/forum/odoo-for-accounting-firms-35/lock-dates-wizard-266980).
- PPN 12 % dengan DPP nilai lain 11/12 (PMK 131/2024, faktur kode 04):
  [DDTC](https://news.ddtc.co.id/berita/nasional/1807950/ppn-dihitung-dengan-dpp-1112-dari-harga-jual-djp-kode-fakturnya-04),
  [DDTC — Coretax](https://news.ddtc.co.id/berita/nasional/1808421/djp-coretax-belum-bisa-hitung-ppn-dengan-dpp-1112-secara-otomatis),
  [Pajak.com](https://www.pajak.com/pajak/pkp-wajib-tahu-praktisi-ini-ungkap-perubahan-penghitungan-ppn-dan-faktur-pajak-di-per-11-2025/amp/).
- `govalues/decimal`: [pkg.go.dev](https://pkg.go.dev/github.com/govalues/decimal); perbandingan dengan
  `shopspring` dan `apd` di catatan riset `go_mysql_patterns.md`.
- Retensi buku 10 tahun (UU KUP Pasal 28 ayat 11): catatan riset `erp_hrmis_wms_indonesia.md`.
