# Rencana — Modul `organization`: perusahaan, cabang, penempatan pengguna

> Disusun 27 Oktober 2026; **dibangun** pada hari yang sama dengan K1–K4 sesuai rekomendasi
> (lihat "Penyesuaian saat dibangun" dan "Bukti" di akhir). Dasar: kernel 6 (lingkup organisasi
> opsi B: nilai lingkup di kernel, strukturnya di modul ini — `PLAN-KERNEL-6.md` dan laporan
> riset `Cakupan kernel 6 platform modular` di repo skill), dan daftar "Batasan yang diketahui
> sampai modul `organization` ada" di `PLAN-KERNEL-6.md`.

## Tujuan dan batas

Satu aplikasi melayani **beberapa perusahaan** (badan hukum) dan **cabang** di tiap perusahaan.
Setiap pengguna **ditempatkan** di perusahaan atau cabang tertentu, dan hanya bisa bekerja di
tempat itu. Kernel 6 sudah menyediakan seluruh jalurnya: `X-Scope` dari web, `d.Scope` yang
memanggil resolver, `kernel.ScopeFrom(ctx)` di setiap request, tugas, dan event, `d.Authz.Units`,
serta slot `shell:scope` di header. Yang belum ada hanyalah pemiliknya. Modul ini:

- memiliki struktur: perusahaan, cabang, dan penempatan pengguna;
- menjadi **resolver lingkup** (`d.Scope.Provide`): menjawab "boleh pengguna ini bekerja di
  perusahaan/cabang ini?" dan memberi zona waktu cabang;
- memberi web **pemilih lingkup** di header dan halaman pengelolaan;
- menutup batasan kernel 6 yang menunggunya (Administrator perusahaan kedua, akun tanpa peran
  lintas perusahaan, daftar pengguna, validasi cabang).

**Bukan** di sini: departemen dan atasan (milik modul HR nanti — ia memakai `Units` lalu
menambah predikat "diri sendiri/bawahan"), konsolidasi laporan dan transaksi antarperusahaan
(milik modul akuntansi), identitas dokumen perusahaan (nama resmi, NPWP, alamat, logo — tetap
di `settings`).

**Tanpa perubahan kernel.** Semua yang dibutuhkan sudah ada di kernel 6 terkunci; kebutuhan
antarmodul lewat port dan event (lihat "Mengapa tanpa kernel request").

## Keputusan yang perlu Anda setujui

| # | Keputusan | Rekomendasi | Alternatif |
|---|---|---|---|
| K1 | Siapa yang boleh **membuat dan menonaktifkan perusahaan** | Izin `organization.companies`, dipegang **seluruh-perusahaan di perusahaan 1** (perusahaan induk pemilik instalasi). Pembuat perusahaan baru otomatis ditempatkan di sana dan menjadi Administrator-nya (lewat event ke `rbac`) | Izin yang sama di perusahaan mana pun — lebih longgar: admin perusahaan B bisa membuat perusahaan C |
| K2 | Pemilik **zona waktu** | `organization`: setiap perusahaan punya zona waktu, cabang boleh menimpanya. Itulah `Scope.Location`, yang dipakai `kernel.Today` | Tetap di profil `settings` — tetapi resolver lingkup lalu bergantung pada modul lain di setiap request |
| K3 | **Pengguna baru** ditempatkan di mana | Otomatis di lingkup tempat ia dibuat: admin di perusahaan B (seluruh perusahaan) membuat pengguna → ia ditempatkan di seluruh B; admin di cabang B-5 → ia ditempatkan di cabang B-5. Lewat event `users.user.created` | Tidak otomatis: admin menempatkannya di halaman Organisasi. Sampai itu, pengguna tidak bisa masuk |
| K4 | **Daftar pengguna** (`users`) per perusahaan | Disaring ke pengguna yang ditempatkan di perusahaan aktif (port opsional di `users`; tanpa `organization`, tidak berubah) | Tetap global untuk semua pemegang `users.read` |

Dokumen di bawah ditulis mengikuti rekomendasi. Bila Anda memilih alternatif, bagian yang
terdampak disebut di tabel "Dampak keputusan" di akhir.

## Ringkasan

| Bagian | Isi |
|---|---|
| Modul baru | `organization` 0.1.0 — fullstack, **tidak** `scoped` (tabel perusahaannya justru lintas perusahaan), menyediakan kernel `scope`, menawarkan `OrganizationClient`, port `UserDirectory` (adapter `users`), memancarkan `organization.company.created`, mendengar `users.user.created` |
| Tabel | `org_companies`, `org_branches`, `org_placements` |
| Izin | `organization.read`, `organization.manage`, `organization.companies` |
| Web | Pemilih lingkup di header (slot `shell:scope`), provider yang membersihkan lingkup basi, halaman Organisasi (perusahaan, cabang) dan Penempatan |
| Modul lain | `users` 0.7.0, `rbac` 0.6.0, `settings` 0.3.0 |
| Pengujian | Tes unit; skenario testbed baru `organization` (MySQL, dua perusahaan, semua modul terpasang) |

## Model data

```sql
-- Perusahaan: badan hukum. Perusahaan 1 dibuat oleh migrasi — data modul yang sudah ada
-- (company_id DEFAULT 1) adalah miliknya.
CREATE TABLE org_companies (
  id BIGINT NOT NULL AUTO_INCREMENT,
  code VARCHAR(20) NOT NULL,          -- huruf besar/angka/-, unik; tampil di pemilih
  name VARCHAR(120) NOT NULL,         -- nama singkat untuk pemilih; nama resmi ada di settings
  timezone VARCHAR(64) NOT NULL,      -- IANA; default APP_TIMEZONE
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY org_companies_code (code)
);

-- Cabang: tempat kegiatan usaha, bukan badan hukum.
CREATE TABLE org_branches (
  id BIGINT NOT NULL AUTO_INCREMENT,  -- unik di seluruh aplikasi: Scope.BranchID
  company_id BIGINT NOT NULL,
  code VARCHAR(20) NOT NULL,
  name VARCHAR(120) NOT NULL,
  timezone VARCHAR(64) NULL,          -- NULL: zona perusahaannya
  nitku CHAR(22) NULL,                -- NPWP pusat 16 digit + nomor urut 6 digit (PMK 112/2022)
  address VARCHAR(255) NOT NULL DEFAULT '',
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY org_branches_code (company_id, code)
);

-- Penempatan: di seluruh perusahaan (branch_id 0) atau di satu cabang.
CREATE TABLE org_placements (
  user_id BIGINT NOT NULL,            -- id di modul users (tanpa foreign key, own-tables-only)
  company_id BIGINT NOT NULL,
  branch_id BIGINT NOT NULL DEFAULT 0,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  created_at DATETIME(6) NOT NULL,
  created_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, company_id, branch_id),
  KEY org_placements_company (company_id, branch_id)
);
```

`branch_id 0` mengikuti konvensi kernel (`Scope.BranchID 0` = seluruh perusahaan) dan `rbac`
(`user_roles.branch_id 0`). Perusahaan dan cabang **dinonaktifkan, tidak dihapus**: data modul
lain menunjuk ID-nya. NITKU divalidasi 22 digit angka; kecocokan 16 digit depannya dengan NPWP di
profil `settings` tidak diperiksa (NPWP milik modul lain).

## Resolusi lingkup (resolver kernel)

`d.Auth.Require` memanggil `Resolve(ctx, userID, X-Scope)` pada **setiap** request yang masuk.

| Diminta | Jawaban |
|---|---|
| `""` (belum memilih) | Penempatan default pengguna. Bila tidak ada yang ditandai: penempatan seluruh-perusahaan dengan `company_id` terkecil, lalu cabang dengan `id` terkecil. Tanpa penempatan aktif → `ErrScopeDenied` |
| `"C"` | `{C, 0}` hanya bila pengguna ditempatkan di **seluruh** perusahaan C — sesuai kontrak kernel ("unit yang diminta kembali persis") |
| `"C:B"` | `{C, B}` bila cabang B milik C, dan pengguna ditempatkan di seluruh C **atau** di cabang B |
| Perusahaan atau cabang nonaktif | `ErrScopeDenied` |
| Format salah, ID tak dikenal | `ErrScopeDenied` (403), bukan 400 — tidak membocorkan apa yang ada |

- **Zona waktu.** `Scope.Location` = zona cabang, lalu zona perusahaan. Lokasi dimuat sekali
  (`time.LoadLocation`) dan disimpan di peta per nama.
- **Biaya per request.** Satu query: penempatan pengguna digabung dengan perusahaan dan cabang
  (indeks PK `org_placements`). Tanpa cache di versi pertama, sehingga menonaktifkan perusahaan
  atau mencabut penempatan langsung berlaku. Cache ditambahkan hanya bila diukur perlu.
- **`Companies(ctx)`** → perusahaan aktif, untuk job yang bekerja per perusahaan.
- **Pemasangan modul ini menyalakan mode beberapa perusahaan.** Sejak itu `kernel.ScopeFrom`
  tanpa lingkup panic. Setiap kode di luar request (job, tugas tanpa lingkup, rute publik) yang
  menyentuh data perusahaan harus memilih perusahaannya sendiri. Lihat "Perubahan di modul lain"
  dan "Risiko".

## Izin

| Izin | Untuk | Catatan |
|---|---|---|
| `organization.read` | Melihat cabang dan penempatan di perusahaan aktif | — |
| `organization.manage` | Mengelola cabang dan penempatan di perusahaan aktif | Cabang baru dan penempatan seluruh-perusahaan butuh izin ini **seluruh-perusahaan** (`Units.AllBranches`). Penempatan di cabang B cukup dengan `Units` yang mencakup B. Tidak ada yang bisa mengubah penempatannya sendiri (mencegah mengunci diri, dan tidak ada jalan pintas ke tempat lain) |
| `organization.companies` | Membuat, mengubah, dan menonaktifkan perusahaan | K1: dipegang seluruh-perusahaan di perusahaan 1. Perusahaan 1 tidak bisa dinonaktifkan |

Penempatan bukan hak akses: ia hanya membuka **pintu** ke sebuah tempat. Apa yang boleh
dilakukan di sana tetap ditentukan peran `rbac`.

## API (`/api/v1/organization`)

| Rute | Izin | Isi |
|---|---|---|
| `GET /me` | masuk | Penempatan saya (perusahaan dan cabang, dengan nama), lingkup aktif, default |
| `PUT /me/default` | masuk | Memilih penempatan default saya |
| `GET /companies` | `organization.companies` | Semua perusahaan (termasuk nonaktif) |
| `POST /companies`, `PUT /companies/{id}` | `organization.companies` | Membuat dan mengubah (kode, nama, zona waktu) |
| `POST /companies/{id}/deactivate`, `/activate` | `organization.companies` | — |
| `GET /branches` | `organization.read` | Cabang perusahaan aktif |
| `POST /branches`, `PUT /branches/{id}`, `…/deactivate`, `…/activate` | `organization.manage` | — |
| `GET /placements?branch=&q=` | `organization.read` | Penempatan di perusahaan aktif, dengan nama dan email (port `UserDirectory`) |
| `POST /placements` | `organization.manage` | `{user_id, branch_id}` |
| `DELETE /placements/{userId}?branch=` | `organization.manage` | — |
| `GET /users?q=` | `organization.manage` | Mencari pengguna untuk ditempatkan |

Audit (`d.Audit`): perubahan perusahaan dan cabang tercatat sebagai `change`; perubahan
penempatan sebagai `security`, karena mengubah siapa bisa masuk ke mana.

## Kontrak, port, event

**Ditawarkan — `contracts.OrganizationClient`:**

```go
type OrganizationClient interface {
	Company(ctx context.Context, id int64) (Company, error)        // ErrNotFound
	Branch(ctx context.Context, id int64) (Branch, error)          // ErrNotFound
	Branches(ctx context.Context, companyID int64) ([]Branch, error)
	// PlacedUserIDs lists the users placed in the company (anywhere in it) — for lists
	// filtered to the active company.
	PlacedUserIDs(ctx context.Context, companyID int64) ([]int64, error)
	// CompaniesOf lists the companies the user is placed in (anywhere in them).
	CompaniesOf(ctx context.Context, userID int64) ([]int64, error)
}
```

Untuk `users` dan `rbac` sekarang, dan nanti untuk modul yang mencetak nama cabang (penjualan)
atau menomori dokumen per cabang (`numbering`).

**Dibutuhkan — port `UserDirectory`** (wajib, adapter `users`): `Get`, `Search`, dan `IDs` (semua
ID pengguna, untuk bootstrap). `IDs` adalah metode baru di `UserClient` `users` 0.7.0.

**Event:**

| Arah | Event | Payload | Untuk |
|---|---|---|---|
| emits | `organization.company.created` v1 | `{company_id, code, name, created_by}` | `rbac` menjadikan pembuatnya Administrator seluruh-perusahaan di sana |
| listens | `users.user.created` (`organization.place-new-user`) | `{id}` dari `users` 0.7.0 | K3: tempatkan di lingkup envelope (lingkup admin yang membuatnya); tanpa lingkup (bootstrap saat start) → seluruh perusahaan 1 |

Event dikirim lewat `d.Tasks` setelah commit, jadi ada jeda singkat (biasanya < 1 detik) sebelum
pengguna baru bisa masuk ke lingkupnya. Halaman Pengguna menampilkan "sedang ditempatkan…" bila
penempatannya belum ada.

## Web

- **Pemilih lingkup** (slot `shell:scope`): "PT A › Cabang Bandung" di header. Daftar diambil dari
  `GET /me`. Memilih memanggil `setScope("C")` atau `setScope("C:B")`, lalu setiap halaman memuat
  ulang (`scope:changed`, `useResource` di `ui-core` 0.4). Tersembunyi bila pengguna hanya punya
  satu penempatan.
- **Provider** (`FrontendModule.provider`): setelah masuk (`session:changed`), memanggil
  `GET /me`. 403 di sini **selalu** berarti lingkup, karena rutenya tidak butuh izin.
  - Ada lingkup tersimpan → lingkup basi (perusahaan dinonaktifkan, penempatan dicabut, tab
    lama): `setScope("")`, lalu coba lagi sekali.
  - Tanpa lingkup tersimpan → pengguna belum ditempatkan di mana pun: halaman penuh "Akun Anda
    belum ditempatkan di perusahaan mana pun. Hubungi administrator."
- **Halaman** (nav "Pengaturan"):
  - `/organization` — perusahaan (`organization.companies`) dan cabang perusahaan aktif;
  - `/organization/placements` — siapa ditempatkan di mana, tambah dan cabut.
- Semua dari kit (`ui-core`, `ui-data`, `ui-forms`, `ui-feedback`), teks Indonesia.

## Bootstrap dan upgrade project yang sudah berjalan

1. Migrasi 001 membuat tabel dan **perusahaan 1** (kode `PUSAT`, nama dari `APP_NAME`, zona
   `APP_TIMEZONE`). Data modul scoped yang sudah ada (`company_id DEFAULT 1`) otomatis miliknya.
2. Saat start, bila `org_placements` masih kosong, setiap pengguna (`UserDirectory.IDs`)
   ditempatkan di **seluruh perusahaan 1**, sebagai default. Setelah upgrade, tidak ada yang
   berubah bagi siapa pun: semua tetap bekerja di perusahaan 1.
3. Administrator yang ada (`rbac`, seluruh perusahaan 1) memegang `*`, termasuk
   `organization.companies`. Ia yang membuat perusahaan berikutnya.

## Perubahan di modul lain

| Modul | Versi | Perubahan |
|---|---|---|
| `users` | 0.7.0 | `UserClient.IDs`; memancarkan `users.user.created` v1 `{id}` di transaksi pembuatan; port opsional `Placements` (adapter `organization`): daftar, pencarian, dan `Search` di kontrak disaring ke pengguna yang ditempatkan di perusahaan aktif (K4). Tanpa port, tidak ada yang berubah |
| `rbac` | 0.6.0 | Mendengar `organization.company.created` → pembuat menjadi Administrator seluruh-perusahaan di perusahaan baru (audit `security`). Port opsional `Placements` (adapter `organization`): `Covers` untuk akun **tanpa peran** butuh hak seluruh-perusahaan di perusahaan tempat akun itu ditempatkan, bukan di perusahaan mana pun; pencarian anggota disaring ke yang ditempatkan di perusahaan aktif. Validasi cabang sudah lewat `d.Scope.Resolve` (0.5.0) — dengan `organization`, kini benar-benar memeriksa |
| `settings` | 0.3.0 | **Wajib sebelum `organization` dipasang:** logo publik (`GET /settings/company/logo`, rute tanpa sesi) memilih perusahaannya sendiri — `?company=<id>`, default 1, lewat `kernel.WithScope` — karena dengan beberapa perusahaan `ScopeFrom` tanpa lingkup panic. Zona waktu di profil dinyatakan usang (K2): formulir tidak menampilkannya lagi, kontrak tetap mengembalikan nilai lamanya. Baris README "Not in this version: several companies" diperbaiki |
| `app-shell`, `ui-core` | — | Tidak berubah (slot dan muat ulang sudah ada) |
| `mailer` | — | Tidak berubah. Kotak keluar tetap lintas perusahaan (batasan tercatat; kandidat `mailer` 0.3 *scoped*) |
| `auth` | — | Tidak berubah. Upaya masuk yang gagal (rute publik) tetap tercatat di perusahaan 1 lewat `audit-log` — catatan sistem |

`organization` butuh `settings` ≥ 0.3.0 bila `settings` terpasang. Manifest belum punya cara
menyatakan versi minimum modul lain (hanya `requires` milik adapter), jadi syarat ini ditulis
di README `organization` dan diperiksa testbed. Taruhannya kecil: dengan `settings` 0.2 hanya
logo publik di halaman masuk yang gagal (500). Mekanisme `requires_modules` di manifest baru
dibuat bila syarat seperti ini muncul lagi.

## Mengapa tanpa kernel request

| Kebutuhan | Dipenuhi dengan |
|---|---|
| Menjawab lingkup per request | `kernel.ScopeResolver` (kernel 6) |
| Zona waktu cabang | `Scope.Location` (kernel 6) |
| Job per perusahaan | `Companies()` (kernel 6) |
| Penempatan untuk `rbac.Covers` dan daftar `users` | Port opsional di konsumen + adapter ke `OrganizationClient` — bukan kernel |
| Administrator perusahaan baru | Event `organization.company.created` → `rbac` |
| Lingkup basi di tab web | Provider modul memanggil `/me` saat boot: 403 di sana pasti soal lingkup |

**Pemicu kernel request di kemudian hari:** web perlu membedakan 403 lingkup dari 403 izin di
rute **mana pun** (mis. kode galat `scope_denied` di respons kernel), atau lingkup perlu sampai
ke rute publik (resolver dari subdomain).

## Pengujian

**Unit** (`kerneltest`, repository tiruan):
- Tabel resolusi di atas, termasuk perusahaan/cabang nonaktif, cabang perusahaan lain, dan default.
- Zona waktu cabang lalu perusahaan.
- Izin: penempatan seluruh-perusahaan butuh `AllBranches`; tidak bisa mengubah penempatan sendiri;
  perusahaan 1 tidak bisa dinonaktifkan.
- Bootstrap sekali saja; penempatan pengguna baru dari lingkup envelope dan tanpa lingkup.

**Testbed — skenario baru `organization`** (MySQL sungguhan; semua modul library terpasang,
termasuk `organization`):

| Pemeriksaan | Cara |
|---|---|
| Upgrade tanpa gangguan | Pasang semua modul, isi data, lalu pasang `organization` → semua pengguna lama bekerja di perusahaan 1 seperti sebelumnya |
| Perusahaan kedua | Admin membuat PT B → ia ditempatkan dan menjadi Administrator di B (event ke `rbac`) |
| Isolasi | Profil `settings` dan berkas `files` B tidak terlihat dari A, dan sebaliknya |
| Penolakan | Pengguna A meminta `X-Scope: B` → 403; cabang B dengan lingkup A → 403; perusahaan dinonaktifkan → 403 |
| Pengguna baru | Admin B membuat pengguna → ditempatkan di B, tidak terlihat di daftar pengguna A |
| Akun tanpa peran | Admin B tidak bisa mereset sandi pengguna tanpa peran yang hanya ditempatkan di A (`Covers` lewat port) |
| Zona waktu | Cabang WITA → `kernel.Today` mengikuti Makassar |
| Mode beberapa perusahaan | Setiap job terdaftar dijalankan (`purge`, `prune`, `rescue`, `cleanup`), setiap rute publik dipanggil (logo, masuk, lupa sandi) — **tidak ada panic** |
| Web | Lingkup basi di `sessionStorage` dibersihkan; pengguna tanpa penempatan melihat pemberitahuan, bukan galat |

## Risiko dan yang harus diverifikasi saat membangun

| Risiko | Penanganan |
|---|---|
| Kode lama memanggil `ScopeFrom` di luar request → panic setelah `organization` dipasang | Sudah ditemukan: logo publik `settings` (lalu `files` di belakangnya) — diperbaiki di `settings` 0.3.0. Skenario testbed menjalankan semua job dan rute publik untuk menemukan sisanya |
| Resolver dipanggil di setiap request | Satu query terindeks; ukur di testbed (latensi `/me` sebelum dan sesudah) |
| Jeda event penempatan pengguna baru | Ditampilkan di UI. Bila mengganggu: port opsional `Placer` di `users` agar penempatan terjadi di transaksi yang sama |
| `audit-log` mencatat catatan sistem (perusahaan 0) sebagai perusahaan 1 | Tetap (batasan `auth` di atas); diputuskan ulang bila ada audit keamanan lintas perusahaan |
| Dua provider web (akses `rbac` dan `organization`) | Provider bersarang menurut urutan registry (`rbac` biasanya di luar). Aman di urutan mana pun: provider `organization` menahan render hanya selama ada lingkup tersimpan yang belum diperiksa, dan provider `rbac` memuat ulang izin pada `scope:changed`. Diperiksa di peramban (lingkup basi "99" jatuh ke default), bukan di testbed |

## Diterima sadar (dari tinjauan)

| Hal | Mengapa diterima |
|---|---|
| Pencarian anggota `rbac` menyaring 50 hasil global (port `UserDirectory` milik `rbac` diimplementasikan juga oleh adapter buatan project; menambah metode memutus mereka) | Instalasi kecil–menengah; pemicu: keluhan "tidak ditemukan" di perusahaan besar → port opsional baru di `rbac` |
| `users.ListIn` memakai satu daftar `IN (...)`: batas MySQL 65.535 parameter | Perusahaan dengan >65 ribu orang |
| Pemeriksaan penempatan `AddMember` di luar transaksi | Celahnya kini butuh wewenang atas akun di kedua perusahaan (`Covers` pada penempatan) |
| Logo publik `settings` dilayani untuk ID perusahaan mana pun, termasuk nonaktif | Logo memang publik |
| Halaman Pengguna (`users.read` dari satu cabang) menampilkan seluruh perusahaan; halaman Penempatan sudah per cabang | Pemicu: kepala cabang yang tidak boleh melihat karyawan cabang lain → `PlacedUserIDs` per cabang di port |
| Pemegang `users.manage` bisa mengetahui sebuah email sudah dipakai di mana pun ("sudah dipakai") | Konsekuensi email unik satu aplikasi |
| Dua Administrator perusahaan X saling mencabut penempatan bersamaan | Dipulihkan dari perusahaan induk ("Tempatkan pengguna…") |
| Baris terlambat yang diedit project: lock mencatat baris baru | Sama dengan perilaku baris blok yang sudah ada; installer memberi catatan |

## Tidak termasuk (sengaja), dengan pemicu

| Tidak | Pemicu |
|---|---|
| Departemen, jabatan, atasan | Modul HR pertama |
| Perusahaan induk–anak, konsolidasi, transaksi antarperusahaan | Modul akuntansi dengan laporan konsolidasi |
| Lingkup dari subdomain untuk rute publik | Klien pertama yang butuh halaman publik per perusahaan |
| Kotak keluar `mailer` per perusahaan | Klien pertama dengan admin perusahaan yang tidak boleh melihat email perusahaan lain |
| Penjaga Administrator aktif terakhir saat **penonaktifan pengguna** (dua admin saling menonaktifkan) | Tetap batasan `rbac` + `users`; bootstrap memulihkan saat start |

## Urutan pengerjaan

1. `settings` 0.3.0 (logo publik eksplisit, zona usang) — prasyarat, bisa dirilis sendiri.
2. `users` 0.7.0 (`IDs`, event `users.user.created`, port `Placements` opsional).
3. `organization` 0.1.0: migrasi, resolver, API, kontrak, event, web.
4. `rbac` 0.6.0 (pendengar `organization.company.created`, port `Placements`).
5. Skenario testbed `organization`; `npm run check`, testbed lengkap.
6. Tinjauan independen (keamanan: penempatan dan pembuatan perusahaan sebagai jalan eskalasi;
   arsitektur), perbaiki, ulangi sampai lulus.
7. Commit setelah Anda setujui.

Sekalian: baris `kernel-alone` di `AGENTS.md` diperbarui. Sejak `be58c23`, sinkronisasi kernel
membawa `CATALOG.md` dan `catalog.json`; tabel aturan belum menyebutnya.

## Dampak keputusan

| Bila memilih alternatif | Yang berubah di rencana |
|---|---|
| K1 longgar | Pemeriksaan `organization.companies` di perusahaan mana pun; risiko: admin satu perusahaan membuat perusahaan lain dan menjadi Administrator-nya |
| K2 di `settings` | Resolver butuh port ke `settings` (dipanggil per request); `settings` 0.3 tidak mengusangkan zona waktu |
| K3 manual | `users` tidak memancarkan event; `organization` tidak mendengarnya; UI Pengguna menampilkan "belum ditempatkan" dengan tautan ke Penempatan |
| K4 global | `users` tanpa port `Placements`; batasan "daftar pengguna lintas perusahaan" tetap tercatat |

## Penyesuaian saat dibangun

| Rancangan | Yang dibangun | Alasan |
|---|---|---|
| `users` memakai port `Placements` biasa ke `organization`, dan `organization` mendengar event `users.user.created` | **Port terlambat** (`register.go.late`): mekanisme installer baru. Baris setter (`usersModule.UsePlacements(...)`) ditulis tepat sebelum penanda akhir registry, setelah semua modul dibangun, dan hanya bila port-nya disambung. `users` menempatkan pengguna baru lewat `PlaceNew` **di transaksi yang sama** dengan pembuatan akunnya; tidak ada event `users.user.created` | `organization` butuh `users` saat dibangun, sedangkan port `users` butuh `organization`: siklus urutan di composition root yang tidak bisa diselesaikan dengan port biasa (installer sendiri mencatat "urutannya urusan project"). Port terlambat menyelesaikannya secara umum, tanpa perubahan kernel; penempatan sinkron juga menghapus jeda event |
| `rbac` memakai port `Placements` biasa | Port terlambat yang sama (`rbacModule.UsePlacements`) | Seragam, dan aman untuk project yang memasang `rbac` sebelum `organization` |
| Pencarian kandidat lewat `UserDirectory.Search` | Hanya orang yang sudah ditempatkan di perusahaan aktif (pencarian sebagian), ditambah **email persis** untuk orang lain | Pencarian sebagian lintas perusahaan membocorkan nama dan email karyawan perusahaan lain |
| — | `auth` 0.7.0: `/auth/me` dan `/auth/password` lewat `d.Auth.Require` kernel (sebelumnya pemeriksa sesi milik `auth` sendiri, tanpa lingkup); guard web menampilkan alasan 403 dengan tombol "Keluar" | Ditemukan testbed: pengguna yang tidak ditempatkan di mana pun tetap lolos `/auth/me` lalu menabrak 403 di setiap halaman tanpa jalan keluar |
| — | Resolver menjawab pengguna tanpa tempat kerja dengan 403 berpesan jelas ("belum ditempatkan…"), bukan pesan umum kernel | Kernel meneruskan `apperror` apa adanya |
| — | `settings` 0.3.0: profil membawa `company_id`, pratinjau logo memakai logo perusahaan itu | Ditemukan saat membangun: halaman pengaturan selalu menampilkan logo perusahaan 1 |
| "Belum ditempatkan di mana pun" | "Tidak bisa bekerja di mana pun": juga yang hanya ditempatkan di perusahaan atau cabang nonaktif | Ditemukan testbed: pengguna seperti itu tidak bisa masuk ke mana pun tetapi tidak muncul di daftar |
| Penempatan cukup dengan `organization.manage` | Menempatkan **dan** mencabut juga butuh wewenang atas akunnya (`d.Authz.Covers`); `rbac` menganggap akun tanpa peran milik **setiap** perusahaan tempat ia ditempatkan (bukan salah satu); halaman Penempatan dari satu cabang hanya menampilkan cabang yang dijangkau | Tinjauan keamanan: admin perusahaan B bisa menempatkan akun tanpa peran milik A di B lalu mengambil alihnya, melindungi karyawan A dari admin A, mengunci admin keluar, dan mengurutkan seluruh daftar pengguna |
| Port terlambat: baris ditulis sebelum penanda akhir | Setiap kali sebuah blok ditambahkan di akhir registry (pemasangan, atau upgrade yang memindahkan blok), **semua** baris terlambat dipindah kembali ke setelahnya; memasang penyedia menulis catatan port opsional yang menunggunya; `--port P=none` melepas port opsional; `upgrade --all --port` hanya meneruskan port yang dideklarasikan tiap modul; katalog menolak port terlambat yang juga dipakai di `after`/`routes` | Tinjauan arsitektur: upgrade yang memindahkan blok `rbac` ke akhir meninggalkan baris terlambatnya di atas blok itu (registry tidak terkompilasi); `organization` tidak bisa dilepas karena port tidak bisa diputus dari CLI |
| Akun tanpa peran tanpa port `Placements` dianggap milik perusahaan aktif | Milik perusahaan induk | Selama port terlambat belum disambung, admin perusahaan lain tidak boleh menjangkau akun tanpa peran (identik untuk aplikasi satu perusahaan) |
| Kandidat lewat email persis untuk siapa pun | Hanya akun yang boleh ditempatkan aktor (`Covers`); pencarian sebagian di antara orang yang ditempatkan (`UserClient.SearchAmong`), bukan 50 hasil global yang disaring | Tidak membocorkan keberadaan akun; pencarian tetap menemukan orang perusahaan ini di instalasi besar |
| `Covers` hanya memakai penempatan untuk akun tanpa peran | Akun **milik** setiap tempat ia memegang peran dan setiap tempat kerjanya (penempatan aktif); `Covers` butuh kedudukan sendiri dan semua yang dipegang akun di setiap tempat itu — seluruh perusahaan atau per cabang. Menempatkan butuh `organization.manage` di setiap tempat kerja akun; mengubah akun butuh `users.manage` di setiap tempat kerjanya. Kepala cabang mengelola akun yang hidup di cabangnya | Tinjauan kedua: peran di B membuat admin B menjangkau akun yang juga bekerja di A; izin baca dianggap wewenang; kepala cabang tidak pernah bisa menempatkan siapa pun |
| Wewenang atas akun memakai penempatan aktif | Memakai **semua** penempatan, aktif atau tidak (`PlacesOf`) | Tinjauan ketiga: menonaktifkan cabang/perusahaan membuat orangnya bisa diambil kepala cabang lain atau perusahaan induk |
| K1: `organization.companies` di perusahaan induk | Ditambah: Administrator seluruh perusahaan induk mengelola seluruh instalasi (memegang semua izin di semua perusahaan; tetap hanya masuk ke tempat ia ditempatkan); jalur pemulihan "Tempatkan pengguna…" di halaman Perusahaan (email persis, dari perusahaan induk) dengan syarat yang sama seperti menempatkan di mana pun — `organization.manage` di perusahaan itu dan wewenang atas akunnya; email tak dikenal dijawab 403 yang sama; diaudit di kedua perusahaan | Tinjauan ketiga: pembuat perusahaan menjadi tak tersentuh oleh Administrator induk; perusahaan yang adminnya saling mencabut penempatan tidak punya jalan kembali |
| `PlaceNew` tanpa lingkup → perusahaan induk | Galat | Tidak ada perusahaan default dalam mode beberapa perusahaan |
| — | `testbed.mjs`: `waitFor` menunggu fungsi async | Ditemukan saat menulis skenario: Promise yang belum selesai dianggap "berhasil" |

## Bukti

Diverifikasi 27 Oktober 2026 pada salinan kerja (belum di-commit), MySQL 8.0 lokal, Go 1.25,
kernel 6 terkunci (`monorepo-elcodelabs/kernel/v6.0`):

| Gerbang | Hasil |
|---|---|
| `catalog.mjs --check` | Lulus (14 modul) |
| Tes unit modul (`go test`, port dan kernel tiruan / `kerneltest`) | Lulus — termasuk resolusi lingkup, penempatan dan wewenang atas akun (lintas perusahaan, cabang, unit nonaktif), pendiri perusahaan, port terlambat `users`/`rbac` |
| Testbed lengkap (7 skenario, `TESTBED_DB_DSN`) | 297 pemeriksaan lulus, 0 gagal. Skenario baru `organization`: pasang semua modul lalu `organization`, sambung port terlambat dengan `module.mjs`, upgrade yang memindahkan blok, modul yang dipasang setelahnya, perusahaan kedua dan Administratornya, isolasi `settings`/`users`, penolakan lingkup, zona cabang, kepala cabang, nonaktif, pemulihan, rute publik dan job tanpa panic, lalu lepas port dan hapus `organization` |
| Peramban (salinan sandbox) | Halaman Cabang, Penempatan, Perusahaan, pemilih lingkup, dialog; lingkup basi dibuang sebelum render |
| Tinjauan independen | 6 putaran (keamanan ×4, arsitektur ×1, verifikasi ×1). Setiap temuan material diperbaiki dan diverifikasi ulang; uji serangan peninjau terakhir (empat skenario terhadap `rbac` sungguhan) kini ditolak. Sisanya tercatat di "Diterima sadar" |
