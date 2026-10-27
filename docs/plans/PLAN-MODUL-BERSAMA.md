# Rencana — Modul bersama ERP dan HRMIS: approvals, notifications, data-import, cetak PDF, fieldcrypt, attachments, payments

> Disusun 27 Oktober 2026. **K1–K12 disetujui sesuai rekomendasi** (27 Oktober 2026); rencana
> berlaku setelah pull request-nya di-review dan digabung. Belum ada kode.
> Dasar: riset kebutuhan pasar dan regulasi (sesi yang sama, ringkasan di STATUS.md › Next),
> riset desain per modul (sumber di akhir), dan pemeriksaan kernel 6 terkunci: container
> Alpine tanpa CGO dengan batas memori 512 MB, filesystem read-only kecuali `DATA_DIR`,
> `d.Tx`, `d.Events`, `d.Tasks` (membawa scope dan pelaku), `d.Jobs`, `request.AllowStream`,
> penjaga lintas situs yang meloloskan panggilan server-ke-server.
> Bergantung pada branch `customers-suppliers` (PR #1): `customers` dan `suppliers` menjadi
> pemakai pertama `data-import` dan `attachments`.

## Tujuan dan batas

Tujuh blok bangunan yang dipakai **sama besar** oleh jalur ERP (penjualan, pembelian) dan HRMIS
(cuti, lembur, penggajian). Setiap blok tidak tahu domain: ia melayani modul lain lewat kontrak,
dan modul lain yang menentukan isi bisnisnya.

| # | Nama | Bentuk | Satu kalimat |
|---|---|---|---|
| 1 | `approvals` | modul fullstack | Persetujuan bertingkat untuk dokumen modul lain sebelum diposting |
| 2 | `notifications` | modul fullstack | Kotak masuk dan lonceng di aplikasi, plus email, dipanggil modul lain |
| 3 | `data-import` | modul fullstack | Impor/ekspor CSV dan XLSX dengan pratinjau, untuk jenis data yang didaftarkan modul lain |
| 4 | `pdfdoc` + `ui-print` | Go kit + tambahan kit UI | PDF dokumen di server (lampiran email, slip terenkripsi) dan cetak dari browser |
| 5 | `fieldcrypt` | Go kit | Enkripsi kolom data spesifik (UU PDP) dengan rotasi kunci dan indeks buta |
| 6 | `attachments` | modul fullstack | Lampiran dan komentar pada record modul mana pun, izinnya mengikuti record induk |
| 7 | `payments` | modul fullstack | Tagihan pembayaran lewat gateway (Midtrans, Xendit): link, QRIS, VA, webhook, rekonsiliasi |

**Bukan** di sini:
- Payroll transfer massal dan pembayaran ke pemasok (payout). Riset menemukan **tidak ada
  spesifikasi publik** format file bank (BCA memakai konverter dan checksum miliknya sendiri), jadi
  modul `payouts` dan kit format bank menunggu template nyata dari proyek pertama.
- Template HTML yang dirender di server. Ini butuh Chromium atau Gotenberg, yang berarti perubahan
  Dockerfile atau compose (lihat K7).
- Mesin workflow umum (BPMN). `approvals` hanya menyetujui atau menolak; status dokumen tetap
  milik `docstate` di modul pemiliknya.
- Terjemahan (i18n). Library berbahasa Indonesia; teks notifikasi disimpan jadi (K5).

**Tanpa perubahan kernel.** Satu catatan untuk kernel nanti: `TMPDIR` ke volume data akan
memudahkan pustaka yang menulis file sementara. Sampai itu, `data-import` memberi direktori
sementaranya sendiri (`DATA_DIR/data-import/tmp`) ke pustaka XLSX.

## Pola yang dipakai ketujuhnya

1. **Pendaftaran (`Declare`) seperti `numbering` dan `period`.** Modul pemakai memanggil
   `Declare(...)` milik penyedia, yang mendaftarkan jenis dokumen, jenis notifikasi, jenis data
   impor, atau jenis record lampiran, beserta fungsi balik (callback) yang berjalan di kode
   pemakai. Penyedia tidak pernah mengimpor pemakai, dan pemakai tidak pernah mengimpor penyedia:
   ia memegang port miliknya sendiri, yang diisi adapter di composition root.
2. **Port ke modul bersama adalah port terlambat (late port).** Modul pemakai yang sudah
   terpasang (`customers`, `suppliers`, nanti `sales`) menerima penyedia lewat setter
   `UseApprovals(…)`, `UseImporter(…)`, dan seterusnya di `register.go.late`. Dengan begitu
   penyedia boleh dipasang sebelum atau sesudah pemakai, dan `module.mjs upgrade <pemakai> --port
   X=<penyedia>` menyambungkannya kapan saja. `Declare` diterima sampai server mulai berjalan.
3. **Semua `scoped`.** `company_id` ada di setiap tabel, kunci, dan query; `branch_id` di tempat
   yang bermakna (permintaan persetujuan, lampiran tidak).
4. **Satu transaksi.** Panggilan antarmodul memakai `ctx` milik pemanggil, sehingga ikut `d.Tx`.
   Notifikasi, tugas impor, dan email ditulis dalam transaksi yang sama: semuanya terjadi hanya
   bila perubahan yang memicunya ter-commit.
5. **Izin milik pemakai.** Penyedia tidak menciptakan izin bisnis. `approvals` memeriksa izin yang
   dideklarasikan pemakai (misalnya `purchasing.approve`), `data-import` memeriksa izin yang
   disebut pemakai untuk jenis datanya, dan `attachments` menanyakan izin baca/tulis record
   kepada pemiliknya. Penyedia hanya punya izin administrasinya sendiri.
6. **Uang lewat kit `money`, data pribadi lewat `fieldcrypt` dan `taxid.Mask`, setiap perubahan
   penting lewat `d.Audit`.**

## Keputusan yang perlu Anda setujui

| # | Keputusan | Rekomendasi | Alternatif | Dasar |
|---|---|---|---|---|
| K1 | Cara `approvals` memberi tahu hasil ke modul pemilik | **Callback dalam transaksi yang sama**: keputusan akhir memanggil `OnDecided` milik pemakai di dalam `d.Tx` keputusan. Bila pemakai menolak (periode ditutup, stok habis), persetujuan ikut batal dan penyetuju melihat alasannya | Event `approvals.request.approved` yang didengar pemakai setelah commit. Lebih longgar, tapi dokumen bisa "disetujui" padahal gagal diposting, dan pemakai wajib memasang `listens` | Kernel: "yang harus benar dalam commit yang sama adalah panggilan port, bukan event" |
| K2 | Rute persetujuan saat kebijakan berubah | **Dibekukan saat diajukan**: langkah dan penyetujunya disalin ke permintaan. Perubahan kebijakan hanya berlaku untuk pengajuan baru | Dihitung ulang setiap kali dibuka | Riset: SAP release strategy, Odoo, pola umum |
| K3 | Dokumen diubah setelah diajukan | **Pemakai mengirim hash isi penting.** Perubahan hash membatalkan permintaan (status `superseded`) dan harus diajukan ulang dari awal. Persetujuan lama tidak terbawa | Mengunci dokumen selama menunggu (cara paling sederhana; pemakai tetap boleh memilihnya) | SAP indikator 4: "persetujuan baru bila nilai berubah" |
| K4 | Menyetujui pengajuan sendiri | **Dilarang secara bawaan** untuk pengaju dan pembuat dokumen, termasuk lewat delegasi. Kebijakan boleh mengizinkan per langkah, dan itu dicatat audit | Diizinkan secara bawaan (bawaan Frappe) | Riset: Frappe mengizinkan secara bawaan dan pernah salah tampil di UI |
| K5 | Teks notifikasi | **Disimpan jadi** (judul dan isi berbahasa Indonesia) bersama kunci jenis dan referensi record | Kunci template + parameter, dirender saat dibaca (perlu bila kelak multibahasa) | Library berbahasa Indonesia; kesederhanaan |
| K6 | Notifikasi baru sampai ke browser | **Polling** jumlah belum dibaca setiap 60 detik dan saat tab kembali aktif, dengan `ETag` (304 bila tak berubah) | SSE lewat `request.AllowStream`. Butuh penyebaran antarkontainer saat deploy (dua kontainer), sehingga ditunda | Riset: SSE tetap perlu polling cadangan karena proxy menahan aliran |
| K7 | Cara membuat PDF | **Go murni**: kit `pdfdoc` di atas `go-pdf/fpdf` (MIT, dirawat di Codeberg) dengan font bawaan PDF, plus `pdfcpu` (Apache-2.0) untuk enkripsi AES-256. Cetak dari browser (`ui-print`) untuk tombol "Cetak" | Chromium/Gotenberg untuk template HTML (perubahan kernel: Dockerfile atau sidecar 0,5–1 GB RAM); maroto v2 (bergantung pada fork gofpdf lama) | Container Alpine tanpa CGO; aturan `text-only` melarang modul membawa file font |
| K8 | `data-import`: modul pusat atau kit per modul | **Modul pusat** dengan pendaftaran: satu tabel log, satu wizard, satu pelaksana tugas. Modul data hanya memberi validasi dan simpan per baris | Kit parser saja, setiap modul membuat endpoint, log, dan halaman impornya sendiri | Pola `Declare`; menghindari tujuh salinan wizard |
| K9 | Mode impor bawaan | **Semua atau tidak sama sekali** (satu `d.Tx`) untuk berkas sampai 5.000 baris. **Lewati baris gagal** (commit per 500 baris, hasil "Sebagian berhasil") boleh dipilih, dan wajib di atas 5.000 baris | Selalu lewati baris gagal | Frappe dan django-import-export menyediakan keduanya |
| K10 | Kunci `fieldcrypt` | **Satu kunci induk dari env** (`FIELDCRYPT_KEYS`, bisa beberapa versi) diturunkan per kolom dengan HKDF. Rotasi lewat kunci baru dan job enkripsi ulang | KMS (Vault/Cloud KMS). Bisa ditambahkan kemudian di tingkat modul tanpa mengubah kernel | OWASP Cryptographic Storage; CipherSweet |
| K11 | Akun gateway pembayaran | **Per perusahaan, disimpan di database**, kunci rahasia dienkripsi `fieldcrypt`. Perusahaan berbeda (badan hukum berbeda) punya akun merchant berbeda | Satu akun per aplikasi lewat env (lebih sederhana, salah untuk grup multi-PT) | Modul `organization`: beberapa badan hukum per aplikasi |
| K12 | Penyedia gateway di 0.1.0 | **Midtrans (Snap) dan Xendit (Payment Sessions)** di balik satu antarmuka penyedia | Satu penyedia dulu | Xendit menandai Invoices v2 sebagai legacy; Payment Sessions adalah API terkininya |

Dokumen di bawah mengikuti rekomendasi. Bila Anda memilih alternatif, bagian terdampak disebut di
tabel "Dampak keputusan".

## Ringkasan rilis

| Rilis | Bentuk | Tabel | Bergantung pada |
|---|---|---|---|
| `fieldcrypt` 0.1.0 | Go kit | — | stdlib saja |
| `pdfdoc` 0.1.0 | Go kit | — | `money`; `codeberg.org/go-pdf/fpdf`, `github.com/pdfcpu/pdfcpu` |
| `ui-core` 0.5.0 | kit UI | — | `PrintLayout` dan konteks record untuk slot |
| `notifications` 0.1.0 | fullstack | `ntf_notifications`, `ntf_preferences` | port opsional `Mailer` → `mailer` |
| `approvals` 0.1.0 | fullstack | `apr_policies`, `apr_policy_steps`, `apr_requests`, `apr_request_steps`, `apr_assignees`, `apr_actions`, `apr_delegations` | port opsional `Notifier` → `notifications`, `Approvers` → `rbac`, `Supervisors` (kosong sampai ada modul `employees`) |
| `data-import` 0.1.0 | fullstack | `imp_jobs`, `imp_job_errors` | port wajib `FileStore` → `files`; opsional `Notifier` |
| `attachments` 0.1.0 | fullstack | `att_links`, `att_comments`, `att_mentions` | port wajib `FileStore` → `files`; opsional `Notifier` |
| `payments` 0.1.0 | fullstack | `pay_accounts`, `pay_intents`, `pay_attempts`, `pay_webhooks` | `money`, `fieldcrypt`; opsional `Notifier` |
| `rbac` 0.7.0 | perubahan | — | kontrak baru `RoleClient.Holders` untuk `approvals` |
| `customers` 0.2.0, `suppliers` 0.2.0 | perubahan | — | port terlambat `Importer` dan `Attachments` |

Nav: halaman pengelolaan di grup "Pengaturan" (order 830 Persetujuan, 840 Impor data, 850
Pembayaran), kotak masuk persetujuan dan notifikasi di header dan menu pengguna.

---

## 1. `approvals`

**Untuk.** Pengajuan dokumen modul lain yang harus disetujui sebelum diposting: PO dan diskon
(ERP), cuti, lembur, dan reimburse (HRMIS). `approvals` hanya menghasilkan "disetujui" atau
"ditolak"; modul pemilik yang memposting.

### Konsep

- **Jenis dokumen** didaftarkan pemakai: `Declare(DocType{Name: "purchasing.order", Label:
  "Pesanan pembelian", ApprovePermission: "purchasing.approve", Fields: [{Name: "amount", Kind:
  Amount, Label: "Total"}], OnDecided: fn})`.
- **Kebijakan** (per perusahaan, per jenis dokumen) diatur admin di halaman Persetujuan. Isinya
  daftar **langkah berurutan**. Setiap langkah punya:
  - **Syarat** (opsional): perbandingan pada field yang didaftarkan, misalnya `amount >= 10000000`.
    Langkah yang syaratnya tidak terpenuhi dilewati.
  - **Penyetuju**:
    - `permission`: siapa pun yang memegang izin persetujuan jenis dokumen itu (atau izin lain
      yang dipilih) di cabang dokumen;
    - `user`: orang tertentu;
    - `supervisor`: atasan pengaju, N tingkat ke atas, lewat port `Supervisors` (kosong sampai ada
      modul `employees`).
  - **Mode**: `any` (penjawab pertama memutuskan), `all` (semua harus setuju; satu tolakan
    menolak langkah), atau `min` (minimal N persetujuan).
  - **Bila tidak ada penyetuju**: `escalate` (ke langkah berikutnya), `admin` (pemegang
    `approvals.manage`), atau `block` (pengajuan ditolak sistem dengan alasan jelas). Tidak pernah
    disetujui diam-diam.
  - **Boleh menyetujui sendiri**: bawaan tidak (K4).
  - **Batas waktu** (opsional, jam): pengingat ke penyetuju, lalu eskalasi bila lewat.
- **Tanpa kebijakan aktif**, jenis dokumen tidak memerlukan persetujuan: `Submit` langsung
  mengembalikan `Approved` dan memanggil `OnDecided` dalam transaksi yang sama. Hasil ini dicatat,
  supaya terlihat bahwa dokumen tidak melewati persetujuan.

### Model data

```sql
CREATE TABLE apr_policies (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  doc_type VARCHAR(100) NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  version INT NOT NULL DEFAULT 1,            -- naik setiap kali langkah diubah
  updated_by BIGINT NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY apr_policies_type (company_id, doc_type)
);

CREATE TABLE apr_policy_steps (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  policy_id BIGINT NOT NULL,
  seq INT NOT NULL,
  label VARCHAR(100) NOT NULL,               -- "Kepala cabang", "Direktur"
  condition_field VARCHAR(50) NOT NULL DEFAULT '',
  condition_op VARCHAR(3) NOT NULL DEFAULT '', -- >=, >, <=, <, =
  condition_value VARCHAR(40) NOT NULL DEFAULT '', -- desimal sebagai teks (exact-numbers)
  approver_kind VARCHAR(12) NOT NULL,        -- permission | user | supervisor
  approver_ref VARCHAR(100) NOT NULL,        -- nama izin, ID pengguna, atau jumlah tingkat
  mode VARCHAR(4) NOT NULL DEFAULT 'any',    -- any | all | min
  min_approvals INT NOT NULL DEFAULT 1,
  on_empty VARCHAR(10) NOT NULL DEFAULT 'escalate', -- escalate | admin | block
  allow_self BOOLEAN NOT NULL DEFAULT FALSE,
  due_hours INT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY apr_policy_steps_seq (company_id, policy_id, seq)
);

-- Satu pengajuan. Rutenya dibekukan di apr_request_steps (K2).
CREATE TABLE apr_requests (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  branch_id BIGINT NOT NULL DEFAULT 0,
  doc_type VARCHAR(100) NOT NULL,
  doc_id BIGINT NOT NULL,                    -- ID primitif di modul pemilik
  doc_title VARCHAR(200) NOT NULL,           -- "PO/2026/10/0012 — CV Sumber Rejeki"
  doc_link VARCHAR(255) NOT NULL,            -- rute web pemilik, mis. /purchasing/orders/12
  doc_hash CHAR(64) NOT NULL,                -- K3
  fields JSON NOT NULL,                      -- nilai field saat diajukan, untuk tampilan dan syarat
  requester_id BIGINT NOT NULL,
  creator_id BIGINT NOT NULL,                -- pembuat dokumen, juga dilarang menyetujui (K4)
  status VARCHAR(12) NOT NULL,               -- pending | approved | rejected | cancelled | superseded
  current_seq INT NOT NULL DEFAULT 0,
  policy_version INT NOT NULL DEFAULT 0,
  previous_id BIGINT NULL,                   -- pengajuan ulang setelah ditolak atau diganti
  pending_key VARCHAR(120) NULL,             -- "<doc_type>:<doc_id>" selama pending, NULL sesudahnya
  created_at DATETIME(6) NOT NULL,
  decided_at DATETIME(6) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY apr_requests_pending (company_id, pending_key), -- satu pengajuan pending per dokumen
  KEY apr_requests_doc (company_id, doc_type, doc_id),
  KEY apr_requests_status (company_id, status, created_at)
);

CREATE TABLE apr_request_steps (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  request_id BIGINT NOT NULL,
  seq INT NOT NULL,
  label VARCHAR(100) NOT NULL,
  approver_kind VARCHAR(12) NOT NULL,
  approver_ref VARCHAR(100) NOT NULL,
  mode VARCHAR(4) NOT NULL,
  min_approvals INT NOT NULL,
  allow_self BOOLEAN NOT NULL,
  status VARCHAR(10) NOT NULL,               -- waiting | active | approved | rejected | skipped
  due_at DATETIME(6) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY apr_request_steps_seq (company_id, request_id, seq)
);

-- Penyetuju yang sudah diketahui saat langkah aktif (user dan supervisor, atau pemegang izin
-- bila port Approvers ada — untuk notifikasi). Untuk approver_kind = permission, hak menyetujui
-- tetap diperiksa langsung dengan d.Authz saat memutuskan.
CREATE TABLE apr_assignees (
  company_id BIGINT NOT NULL,
  step_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  delegated_from BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (company_id, step_id, user_id)
);

-- Riwayat, hanya ditambah. Satu suara per orang per langkah.
CREATE TABLE apr_actions (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  request_id BIGINT NOT NULL,
  step_id BIGINT NOT NULL DEFAULT 0,         -- 0 untuk submit, cancel, supersede
  actor_id BIGINT NOT NULL,
  on_behalf_of BIGINT NOT NULL DEFAULT 0,    -- delegasi
  action VARCHAR(12) NOT NULL,               -- submit | approve | reject | cancel | supersede | escalate | remind
  vote_key VARCHAR(60) NULL,                 -- "<step_id>:<actor_id>" untuk approve/reject, NULL selain itu
  comment VARCHAR(1000) NOT NULL DEFAULT '',
  acted_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY apr_actions_vote (company_id, vote_key),
  KEY apr_actions_request (company_id, request_id)
);

CREATE TABLE apr_delegations (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  from_user BIGINT NOT NULL,
  to_user BIGINT NOT NULL,
  doc_type VARCHAR(100) NOT NULL DEFAULT '', -- '' = semua jenis
  starts_on DATE NOT NULL,
  ends_on DATE NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY apr_delegations_from (company_id, from_user, ends_on)
);
```

MySQL tidak punya indeks parsial. Kolom `pending_key` (diisi saat pending, NULL sesudahnya)
ditambah `UNIQUE` memberi "satu pengajuan pending per dokumen", karena MySQL mengizinkan banyak
NULL. Pola yang sama dipakai `vote_key`.

### Kontrak

```go
// ApprovalClient is the approvals module's public contract.
type ApprovalClient interface {
	// Declare registers a document type in the consumer's New (or its late setter).
	Declare(t DocType)
	// Submit asks for approval inside the consumer's d.Tx. With no active policy, or every step
	// skipped, it returns Approved and calls OnDecided in the same transaction.
	Submit(ctx context.Context, s Submission) (Result, error)
	// Supersede ends a pending request because the document changed (K3); the consumer submits again.
	Supersede(ctx context.Context, docType string, docID int64) error
	// Cancel withdraws a pending request — the consumer calls it when its user cancels the document.
	Cancel(ctx context.Context, docType string, docID int64, reason string) error
	// Status is the latest request of a document, for its page (none: Status.None).
	Status(ctx context.Context, docType string, docID int64) (Status, error)
}

type DocType struct {
	Name, Label       string
	ApprovePermission string  // the consumer's own permission, declared by the consumer
	Fields            []Field // what policies may test (amount, discount_pct, days)
	// OnDecided runs inside the deciding transaction when a request is approved or rejected. An
	// error rolls the decision back and is shown to the approver.
	OnDecided func(ctx context.Context, docID int64, outcome Outcome, comment string) error
}

type Submission struct {
	DocType   string
	DocID     int64
	BranchID  int64
	Title     string
	Link      string
	Hash      string            // of the fields that, if changed, need a new approval
	Values    map[string]string // field name → value (decimal as text)
	CreatorID int64             // who made the document; RequesterID is the signed-in user
}
```

### Keputusan menyetujui (`Decide`)

Dalam satu `d.Tx`:
1. Kunci baris permintaan (`FOR UPDATE`).
2. Pastikan statusnya `pending` dan langkah yang dituju `active` (bila tidak: 409, "sudah
   diputuskan orang lain").
3. Pastikan pelaku berhak:
   - `permission`: `d.Authz.Units(izin)` mencakup cabang dokumen;
   - `user` atau `supervisor`: terdaftar di `apr_assignees`, atau penerima delegasi aktif dari
     orang yang terdaftar.
4. Tolak bila pelaku adalah pengaju atau pembuat (kecuali `allow_self`).
5. Catat suara. Indeks unik `vote_key` menolak suara kedua.
6. Hitung hasil langkah:
   - `any`: suara pertama memutuskan;
   - `all`: satu tolakan menolak, langkah selesai saat semua penyetuju yang diketahui setuju;
   - `min`: N persetujuan.
7. Langkah disetujui: aktifkan langkah berikutnya yang syaratnya terpenuhi. Tidak ada lagi:
   permintaan `approved`, panggil `OnDecided`. Ditolak: permintaan `rejected` (komentar wajib),
   panggil `OnDecided`.
8. Kirim notifikasi lewat port `Notifier`, lalu catat audit.

Mode `all` dengan approver `permission` tanpa port `Approvers` tidak bisa mengetahui "semua"
pemegang izin. Kombinasi itu ditolak saat kebijakan disimpan.

### Permintaan via email dan keamanan

Email notifikasi hanya membawa tautan ke halaman permintaan di aplikasi (perlu sesi masuk).
Keputusan selalu `POST` dari halaman itu, tidak pernah lewat tautan `GET` bertoken. Riset
menemukan pemindai tautan email (Defender, Proofpoint) pernah "menyetujui" permintaan lewat
tautan seperti itu.

### Izin, API, web

- **Izin:**
  - `approvals.manage`: kebijakan dan delegasi milik siapa pun, di seluruh perusahaan;
  - `approvals.view_all`: melihat semua permintaan.
  - Menyetujui memakai izin milik pemakai. Setiap orang melihat permintaannya sendiri dan yang
    menunggu dirinya.
- **API** (`/api/v1/approvals`):
  - `GET /inbox`: menunggu saya;
  - `GET /requests?mine=1|status=`;
  - `GET /requests/{id}`: dengan rute dan riwayat;
  - `POST /requests/{id}/approve` dan `/reject` (`{comment}`);
  - `GET`/`PUT /policies/{docType}`;
  - `GET`/`POST`/`DELETE /delegations`.
- **Web:**
  - halaman "Persetujuan" dengan tab Menunggu saya, Pengajuan saya, Semua;
  - detail permintaan dengan rute langkah (`Stepper` dari `ui-forms`), riwayat, serta tombol
    Setujui/Tolak (`ReasonDialog` untuk tolak);
  - halaman "Pengaturan › Persetujuan" untuk kebijakan per jenis dokumen;
  - komponen status persetujuan di slot generik `record:status` (lihat "Slot record"), tampil di
    halaman dokumen modul pemilik tanpa saling import.
- **Job:** `approvals.remind` setiap jam (pengingat dan eskalasi lewat `due_at`, per
  perusahaan lewat `d.Scope.Companies`).

### Audit

Setiap pengajuan, keputusan, pembatalan, penggantian, dan perubahan kebijakan dicatat. Perubahan
kebijakan dan delegasi dicatat sebagai `security`, karena mengubah siapa yang boleh menyetujui.

---

## 2. `notifications`

**Untuk.** Pesan yang ditujukan ke orang tertentu: "PO menunggu persetujuan Anda", "Cuti Anda
disetujui", "Impor selesai: 3 baris gagal", "Pembayaran INV/… diterima".

### Konsep

- **Jenis notifikasi** didaftarkan pemakai: `Declare(Kind{Name: "approvals.assigned", Label:
  "Persetujuan menunggu", Category: "action", Email: EmailDefaultOn})`.
  - Kategori `action` (perlu tindakan) mengirim email secara bawaan.
  - Kategori `info` hanya di aplikasi kecuali pengguna menyalakan email.
- **`Notify`** dipanggil pemakai dalam transaksinya. Isinya: daftar penerima (ID pengguna), jenis,
  `thread_key`, judul, isi singkat, tautan, dan kunci idempotensi.
- **Penggabungan**: satu baris per (pengguna, `thread_key`). Notifikasi kedua untuk thread yang sama
  menaikkan `times`, memperbarui judul, dan menandainya belum dibaca lagi. Ini mencegah banjir
  notifikasi (pola GitHub).
- **Idempotensi**: kunci per penerima. Pemanggilan ulang (tugas yang diulang) tidak menambah apa pun.
- **Pelaku tidak diberi tahu tentang aksinya sendiri.**
- **Email**: lewat port `Mailer` (adapter ke `mailer`), di transaksi yang sama, sehingga
  `mailer` mengirimnya hanya setelah commit. Email memuat judul, isi, tautan ke aplikasi, dan
  tautan "berhenti menerima email jenis ini" yang membuka halaman preferensi (perlu masuk).
- **Izin saat dibuka**: daftar hanya menampilkan judul yang disimpan. Membuka tautan menuju halaman
  modul pemilik, yang memeriksa izinnya sendiri. Judul tidak boleh berisi data rahasia; ini
  ditulis di README sebagai kewajiban pemakai.

### Model data

```sql
CREATE TABLE ntf_notifications (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  kind VARCHAR(100) NOT NULL,
  thread_key VARCHAR(150) NOT NULL,         -- "approvals:request:12"
  title VARCHAR(200) NOT NULL,
  body VARCHAR(500) NOT NULL DEFAULT '',
  link VARCHAR(255) NOT NULL DEFAULT '',
  actor_id BIGINT NOT NULL DEFAULT 0,
  times INT NOT NULL DEFAULT 1,             -- berapa kejadian digabung di thread ini
  last_key VARCHAR(150) NOT NULL DEFAULT '', -- kunci idempotensi terakhir
  read_at DATETIME(6) NULL,
  archived_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY ntf_thread (company_id, user_id, thread_key),
  KEY ntf_unread (company_id, user_id, read_at, updated_at)
);

-- Hanya yang diubah pengguna; bawaan datang dari Declare.
CREATE TABLE ntf_preferences (
  company_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  kind VARCHAR(100) NOT NULL,
  email BOOLEAN NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (company_id, user_id, kind)
);
```

Kunci idempotensi per penerima dicek dengan `last_key` pada baris thread. Pemanggil memakai kunci
yang unik untuk satu kejadian, misalnya `approvals:step:41:assigned`. Kejadian baru pada thread
yang sama selalu membawa kunci baru.

### Kontrak

```go
// NotificationClient is the notifications module's public contract.
type NotificationClient interface {
	Declare(k Kind)
	// Notify stores one notification per recipient (merged by thread) and queues email where
	// the recipient's preference says so — inside the caller's d.Tx. The actor is skipped.
	Notify(ctx context.Context, n Notification) error
}

type Notification struct {
	Kind           string
	Recipients     []int64
	ThreadKey      string
	Title, Body    string // Indonesian, no secrets: shown in the list
	Link           string // the owner's page, which checks access itself
	IdempotencyKey string
}
```

### API, web, job

- **API** (`/api/v1/notifications`):
  - `GET /` berhalaman, `?unread=1`;
  - `GET /count`: angka maksimal 99, dengan `ETag`;
  - `POST /read` (`{ids}` atau `{before}`);
  - `POST /{id}/archive`;
  - `GET`/`PUT /preferences`.
- **Web:**
  - lonceng di slot `header-end` dengan angka, panel daftar singkat, dan "Tandai semua dibaca";
  - halaman "Notifikasi";
  - halaman preferensi lewat `user-menu:items`.
- **Job:** `notifications.prune` harian. Hapus yang sudah dibaca setelah 90 hari, semua setelah 365
  hari (env `NOTIFICATIONS_RETENTION_DAYS`).
- **Izin:** tidak ada izin bisnis. Setiap pengguna hanya melihat miliknya, di perusahaan aktif.

---

## 3. `data-import`

**Untuk.** Memindahkan ribuan baris dari Excel atau sistem lama, dan mengunduh data ke Excel:
pelanggan, pemasok, barang, karyawan, saldo cuti awal, komponen gaji.

### Konsep

- **Jenis data** didaftarkan pemakai:

```go
type Entity struct {
	Name       string     // "customers.customer"
	Label      string     // "Pelanggan"
	Permission string     // the consumer's, e.g. "customers.manage"
	Columns    []Column   // name, label, required, kind (text|decimal|date|bool|enum), example
	Key        string     // natural key for update/upsert, e.g. "code"
	// Validate checks one row without writing (the preview); returns field messages.
	Validate func(ctx context.Context, row Row) []FieldError
	// Apply writes one row inside the given transaction; mode is insert, update, or upsert.
	Apply func(ctx context.Context, row Row, mode Mode) (Outcome, error)
	// Export streams rows for the download (nil: no export).
	Export func(ctx context.Context, filter map[string]string, emit func(Row) error) error
}
```

- **Alur**:
  1. Unduh template: XLSX, satu lembar, kolom wajib bertanda, baris contoh, lembar "Petunjuk".
  2. Unggah: disimpan lewat `files`, dibatasi ukurannya.
  3. Petakan kolom: otomatis menurut judul, bisa diubah.
  4. Pratinjau: tugas latar belakang menjalankan `Validate` di setiap baris tanpa menulis apa pun;
     hasilnya jumlah baris baru, diperbarui, dan gagal, beserta daftar kesalahan per baris.
  5. Konfirmasi mode (insert, update, upsert) dan cara gagal (K9).
  6. Jalankan sebagai tugas `LaneLong`.
  7. Hasil: log per baris dan unduhan "baris gagal" yang sudah diberi kolom pesan, untuk diperbaiki
     dan diunggah lagi.
- **Membaca berkas**:
  - XLSX dengan `excelize` lewat iterator `Rows()` (streaming), dengan `UnzipSizeLimit` 200 MB,
    `UnzipXMLSizeLimit` 16 MB, dan `TmpDir` di `DATA_DIR/data-import/tmp` (filesystem kontainer
    read-only).
  - CSV dengan deteksi BOM dan UTF-8 (selain itu dibaca sebagai Windows-1252), deteksi pemisah
    `;`/`,`/tab dari baris judul (Excel berbahasa Indonesia memakai `;`), dan baris `sep=` dihormati.
- **Angka dan tanggal**:
  - Lokal ditentukan per berkas. Bawaan Indonesia: `1.234.567,89` dan `DD/MM/YYYY`.
  - Tanggal XLSX dibaca dari nilai serialnya.
  - Angka menjadi teks desimal untuk kit `money`, tidak pernah float (`exact-numbers`).
- **Ekspor**:
  - XLSX lewat `StreamWriter`, semua sel ditulis sebagai teks atau angka, tidak pernah rumus.
  - CSV UTF-8 dengan BOM. Sel yang diawali `= + - @`, tab, atau CR, termasuk versi lebar penuhnya,
    diberi awalan `'` (OWASP CSV injection).
  - Setiap ekspor dicatat `AuditExport`.
- **Izin**: diperiksa dua kali. Saat mengunggah (izin jenis data milik pemakai) dan saat tugas
  berjalan, karena tugas membawa pelaku dan scope (kernel 6).
- **Notifikasi** "Impor selesai" lewat port `Notifier`. Selama impor, pemakai tidak boleh
  mengirim notifikasi per baris.

### Model data

```sql
CREATE TABLE imp_jobs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  entity VARCHAR(100) NOT NULL,
  file_id BIGINT NOT NULL,                  -- di modul files
  file_name VARCHAR(255) NOT NULL,
  mapping JSON NOT NULL,                    -- kolom berkas → kolom entitas
  locale VARCHAR(10) NOT NULL DEFAULT 'id',
  mode VARCHAR(8) NOT NULL DEFAULT 'insert', -- insert | update | upsert
  atomic BOOLEAN NOT NULL DEFAULT TRUE,     -- K9
  status VARCHAR(12) NOT NULL,              -- uploaded | previewing | previewed | running | done | partial | failed
  rows_total INT NOT NULL DEFAULT 0,
  rows_new INT NOT NULL DEFAULT 0,
  rows_updated INT NOT NULL DEFAULT 0,
  rows_failed INT NOT NULL DEFAULT 0,
  created_by BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  finished_at DATETIME(6) NULL,
  PRIMARY KEY (id),
  KEY imp_jobs_entity (company_id, entity, created_at)
);

CREATE TABLE imp_job_errors (
  company_id BIGINT NOT NULL,
  job_id BIGINT NOT NULL,
  row_no INT NOT NULL,
  field VARCHAR(50) NOT NULL DEFAULT '',
  message VARCHAR(500) NOT NULL,
  PRIMARY KEY (company_id, job_id, row_no, field)
);
```

Batas: 50.000 baris dan 20 MB per berkas (env), 500 kesalahan disimpan per tugas (sisanya
dihitung saja). Berkas unggahan dihapus dari `files` 30 hari setelah tugas selesai.

### API, web

- **API** (`/api/v1/data-import`):
  - `GET /entities`: jenis data yang boleh diakses pengguna;
  - `GET /entities/{entity}/template`;
  - `POST /jobs` (multipart);
  - `PUT /jobs/{id}/mapping`;
  - `POST /jobs/{id}/run`;
  - `GET /jobs/{id}` dan `GET /jobs/{id}/errors.xlsx`;
  - `POST /entities/{entity}/export`.
- **Web:**
  - halaman "Impor data" berisi riwayat dan wizard 4 langkah (`Stepper`, `FileDropzone`,
    `DataTable`);
  - tombol "Impor" dan "Ekspor" di slot generik `list:actions`; halaman daftar modul pemakai
    menyebut jenis datanya lewat `RecordListProvider` (lihat "Slot record").

---

## 4. `pdfdoc` (Go kit) dan `ui-print` (kit UI)

**Untuk.** Faktur, surat jalan, PO, slip gaji, dan surat keterangan sebagai PDF di server (untuk
lampiran email, arsip, atau slip berpassword), dan tombol "Cetak" di browser.

**Mengapa bukan modul.** Data kop surat sudah dimiliki `settings` (nama, nama resmi, alamat, NPWP,
logo), dan isi dokumen dimiliki modul pemilik. Yang dibutuhkan bersama hanya cara menyusun halaman.
Itu kode tanpa tabel, sehingga berbentuk kit.

### `pdfdoc` (Go kit, `go_dependencies`: `codeberg.org/go-pdf/fpdf`, `github.com/pdfcpu/pdfcpu`)

```go
doc := pdfdoc.New(pdfdoc.A4, pdfdoc.Letterhead{Name, LegalName, Address, NPWP, Logo /* PNG/JPEG bytes */})
doc.Title("FAKTUR", "INV/2026/10/0012")
doc.Parties(pdfdoc.Party{Label: "Kepada", Lines: …}, pdfdoc.Party{Label: "Tanggal", Lines: …})
doc.Table(columns, rows)                  // memecah halaman sendiri, judul kolom diulang
doc.Totals([]pdfdoc.Total{{"Subtotal", amt}, {"PPN 12%", ppn}, {"Total", total}})
doc.Words(total)                           // "Terbilang: satu juta dua ratus ribu rupiah"
doc.Signatures("Hormat kami", "Penerima")
b, err := doc.Bytes()                      // nomor halaman "Halaman 1 dari 2" otomatis
b, err = pdfdoc.Encrypt(b, userPassword, ownerPassword) // AES-256 lewat pdfcpu
```

- **Font**: font bawaan PDF (Helvetica) dengan penerjemah cp1252, yang mencakup bahasa Indonesia.
  Modul tidak membawa file font (`text-only`). Rp, tanda pisah, dan huruf beraksen diuji di tes.
- **Logo**: PNG atau JPEG. Logo WebP dari `settings` dikonversi lewat `golang.org/x/image/webp`,
  atau dilewati dengan catatan. Ini diputuskan saat membangun.
- **Terbilang**: fungsi `Words(money.Amount)` berbahasa Indonesia, sampai triliun, dengan sen
  bila ada. Diuji dengan tabel kasus.
- **Batas memori**: satu dokumen sampai beberapa ratus baris. Dokumen yang lebih besar (laporan)
  bukan untuk kit ini.
- **Password slip gaji**: kit menolak password kurang dari 8 karakter. README memperingatkan agar
  tidak memakai tanggal lahir saja.

### `ui-print` (komponen di `ui-core` 0.5.0)

- `PrintLayout`: kop, judul, blok pihak, tabel, total, tanda tangan.
- `PrintButton`: `window.print()`.
- CSS `@media print`: menyembunyikan shell, mengatur ukuran kertas A4, tidak memotong baris tabel.
- Nilai uang memakai komponen `Money` yang sudah ada. Contoh di `ui-gallery`.

---

## 5. `fieldcrypt` (Go kit, standard library saja)

**Untuk.** Data spesifik UU PDP yang disimpan modul: data kesehatan, biometrik, nomor rekening
(pemasok, karyawan), kunci rahasia gateway pembayaran (K11).

```go
ring, err := fieldcrypt.ParseKeys(config.Require("FIELDCRYPT_KEYS")) // "k2:base64,k1:base64", pertama = aktif
f := ring.Field("supp_bank_accounts.account_number")                  // subkunci HKDF per kolom
ct, err := f.Encrypt(plain, ad)       // ad: "<company_id>|<row_id>" mengikat nilai ke barisnya
pt, err := f.Decrypt(ct, ad)
idx := ring.Index("supp_bank_accounts.account_number", 32).Of(normalized) // indeks buta, bit dipotong
stale := f.NeedsRotation(ct)          // kunci lama → job enkripsi ulang di modul
```

- **Format**: `v1|key-id|nonce(12)|ciphertext+tag`, dengan AES-256-GCM lewat
  `cipher.NewGCMWithRandomNonce` (Go 1.24+). Disimpan sebagai `VARBINARY`.
- **Data tambahan (AD)** mengikat nilai ke perusahaan dan barisnya, supaya tidak bisa disalin ke
  baris lain. Modul yang memakai `row_id` menulis baris dulu, lalu mengenkripsi dengan ID-nya
  dalam `d.Tx` yang sama.
- **Kunci** dari env (`FIELDCRYPT_KEYS`). Setiap modul pemakai mendeklarasikan env yang sama
  (installer menuliskannya sekali). Subkunci enkripsi dan subkunci indeks diturunkan terpisah.
  - Batas 2³² enkripsi per kunci: README menganjurkan rotasi tahunan.
- **Indeks buta**: HMAC-SHA256 atas nilai yang dinormalisasi, dipotong ke jumlah bit yang dipilih,
  supaya ada tabrakan yang disengaja (pola CipherSweet). Tidak boleh untuk kolom berentropi rendah.
  Tidak bisa dipakai untuk `LIKE` atau pengurutan.
- **Tidak pernah**: enkripsi deterministik atas nilainya, nonce tetap, kunci di database atau
  git, atau nilai asli di log dan audit.

**Pemakaian pertama** (di luar 0.1.0 kit, sebagai rilis modul terpisah): `suppliers` 0.3.0
mengenkripsi `account_number`, dengan migrasi data lewat job. `payments` memakainya sejak awal.

---

## 6. `attachments` (lampiran dan komentar)

**Untuk.** Surat dokter di pengajuan cuti, bukti di reimburse, kontrak di pemasok, diskusi di PO.

### Konsep

- **Jenis record** didaftarkan pemakai:

```go
type RecordType struct {
	Name  string // "suppliers.supplier"
	Label string
	// Access answers for the signed-in user: may they read the record, may they add or remove
	// attachments and comments, and is the record locked (posted: nothing may be removed).
	Access func(ctx context.Context, recordID int64) (read, write, locked bool, err error)
	Allowed  []string // content types (nil: files' defaults)
	MaxBytes int64
}
```

- **Setiap akses bertanya ke pemilik record**: daftar, unduh, unggah, komentar, hapus. Jenis yang
  tidak terdaftar ditolak. Ini menutup celah IDOR (mengunduh lampiran record yang tidak boleh
  dilihat).
- **Berkas disimpan oleh `files`**. `attachments` hanya menyimpan tautan (record ↔ `file_id`).
  Unggahan di-stream ke disk, tipe ditentukan dari isi, SVG/HTML ditolak. Unduhan selalu sebagai
  lampiran dengan `nosniff`.
- **Record yang dikunci** (sudah diposting): lampiran boleh ditambah, tidak boleh dihapus.
  `attachments` juga memanggil `files.Lock`, sehingga berkasnya tidak bisa dihapus dari mana pun.
- **Komentar**:
  - teks biasa sampai 2.000 karakter, ditampilkan ter-escape;
  - penyebutan `@[user:12]` dipilih dari daftar (pencarian pengguna lewat port `UserDirectory`);
  - orang yang disebut diberi tahu hanya bila ia boleh membaca record itu (dicek lewat `Access`
    atas namanya, dengan `kernel.WithActor`). Bila pengecekan atas nama orang lain tidak bisa
    dilakukan, penyebutan hanya memberi tahu orang yang sudah terlibat. Ini diverifikasi saat
    membangun (lihat Risiko).
  - komentar bisa diubah oleh penulisnya dalam 15 menit; dihapus secara lunak (isi diganti
    "Komentar dihapus"); setiap perubahan diaudit.

### Model data

```sql
CREATE TABLE att_links (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  record_type VARCHAR(100) NOT NULL,
  record_id BIGINT NOT NULL,
  file_id BIGINT NOT NULL,                  -- di modul files
  file_name VARCHAR(255) NOT NULL,
  content_type VARCHAR(100) NOT NULL,
  size BIGINT NOT NULL,
  uploaded_by BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  removed_at DATETIME(6) NULL,
  removed_by BIGINT NULL,
  PRIMARY KEY (id),
  KEY att_links_record (company_id, record_type, record_id)
);

CREATE TABLE att_comments (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  record_type VARCHAR(100) NOT NULL,
  record_id BIGINT NOT NULL,
  author_id BIGINT NOT NULL,
  body VARCHAR(2000) NOT NULL,
  edited_at DATETIME(6) NULL,
  deleted_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY att_comments_record (company_id, record_type, record_id, created_at)
);

CREATE TABLE att_mentions (
  company_id BIGINT NOT NULL,
  comment_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  PRIMARY KEY (company_id, comment_id, user_id)
);
```

### API, web

- **API** (`/api/v1/attachments`). Setiap rute membawa `{type}/{id}` record:
  - `GET /{type}/{id}`: lampiran dan komentar;
  - `POST /{type}/{id}/files` (multipart);
  - `GET /{type}/{id}/files/{linkId}`: unduh;
  - `DELETE /{type}/{id}/files/{linkId}`;
  - `POST /{type}/{id}/comments`, `PUT .../comments/{cid}`, `DELETE .../comments/{cid}`.
- **Web:** komponen lampiran dan komentar dalam satu linimasa, di slot generik `record:activity`.
  Komponen ini membaca `type` dan `id` record dari `useRecord()` (lihat "Slot record"). Unggahan
  memakai `FileDropzone` dari `ui-forms`.
- **Izin:** tidak ada izin sendiri selain `attachments.manage` untuk melihat semua lampiran
  perusahaan (pemeriksaan administrasi). Semua lainnya mengikuti `Access` milik pemilik record.

---

## 7. `payments`

**Untuk.** Menagih pembayaran dokumen modul lain (faktur penjualan, tagihan iuran, booking) lewat
gateway: payment link, QRIS dinamis, Virtual Account, e-wallet. Hasilnya diteruskan ke modul
pemilik dalam transaksi yang sama.

### Konsep

- **Akun gateway per perusahaan** (K11): penyedia (`midtrans`/`xendit`), lingkungan
  (`sandbox`/`production`), kunci rahasia terenkripsi `fieldcrypt`, dan `webhook_key` acak untuk
  URL webhook-nya. Rahasia tidak pernah dikembalikan oleh API (hanya "terisi").
- **Jenis tagihan** didaftarkan pemakai: `Declare(Source{Name: "sales.invoice", OnPaid: fn})`.
  `OnPaid(ctx, sourceID, paidTotal, status)` berjalan dalam transaksi yang mencatat pembayaran.
- **Intent dan attempt**:
  - satu **intent** per tagihan: sumber, referensi tampilan, jumlah, terbayar, status `open`,
    `partially_paid`, `paid`, `overpaid`, atau `canceled`;
  - setiap link yang dibuat adalah **attempt** dengan `order_id` unik
    (`<prefix>-<intentId>-<n>`), karena Midtrans menolak `order_id` yang pernah dipakai.
- **Jumlah**: gateway menerima rupiah bulat. Intent dengan sen ditolak (422). Biaya MDR dicatat
  terpisah dari jumlah bruto.
- **Webhook**:
  - Route publik `POST /api/v1/payments/webhooks/{provider}/{companyId}/{webhookKey}`, tanpa sesi.
  - Penjaga lintas situs kernel meloloskan panggilan server-ke-server.
  - Alurnya:
    1. Cari akun dari `companyId` dan `webhookKey`, lalu kerjakan sisanya dalam scope perusahaan itu.
    2. Verifikasi. Midtrans: `SHA512(order_id+status_code+gross_amount+ServerKey)` atas string
       mentah. Xendit: header `x-callback-token`. Keduanya dibandingkan dengan waktu konstan.
    3. Simpan body mentah ke `pay_webhooks` dengan kunci dedupe (`INSERT IGNORE`).
    4. Antrekan tugas dengan scope perusahaan akun itu.
    5. Jawab 200 segera.
  - Tugas **mengambil ulang status dari API gateway** (Get Status / GET session), tidak
    memercayai isi webhook atau redirect browser. Lalu menerapkan mesin status satu arah (`paid`
    tidak kembali ke `pending`), mencocokkan jumlah, memperbarui intent, dan memanggil `OnPaid`.
- **Cadangan bila webhook hilang**: job setiap 10 menit menanyakan attempt `pending` yang lebih
  dari 15 menit tanpa kabar, dan menandai yang lewat `expires_at` sebagai `expired`.
- **Rekonsiliasi**: job harian membandingkan attempt dengan status gateway dan membuat laporan
  selisih di halaman Pembayaran.
- **Bayar sebagian dan lebih bayar**: jumlah attempt yang dibayar dijumlahkan. Lebih bayar ditandai
  dan tidak pernah dikembalikan otomatis. Refund adalah fitur versi berikutnya.

### Penyedia

```go
// Provider is one gateway, behind the module's own interface (infrastructure).
type Provider interface {
	CreateCheckout(ctx context.Context, acc Account, a Attempt) (url string, expires time.Time, err error)
	FetchStatus(ctx context.Context, acc Account, orderID string) (Status, error)
	VerifyWebhook(acc Account, r WebhookRequest) (orderID, dedupeKey string, err error)
}
```

- **Midtrans**: Snap (`POST /snap/v1/transactions`) dengan `enabled_payments` yang dipilih per
  akun, `expiry` kustom, dan `redirect_url` sebagai link. Lunas berarti `status_code 200` dan
  `settlement`/`capture` dengan `fraud_status` `accept` bila ada. Payment Link API masih beta,
  jadi tidak dipakai.
- **Xendit**: Payment Sessions (`POST /sessions`, `mode: PAYMENT_LINK`), yang menggantikan
  Invoices v2 (legacy). Lunas berarti `COMPLETED`.
- HTTP keluar memakai `net/http` dengan batas waktu 15 detik. Tes memakai server tiruan
  (`httptest`), tidak memanggil gateway sungguhan.

### Model data

```sql
CREATE TABLE pay_accounts (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  provider VARCHAR(12) NOT NULL,            -- midtrans | xendit
  environment VARCHAR(10) NOT NULL,         -- sandbox | production
  secret VARBINARY(512) NOT NULL,           -- fieldcrypt: server key / secret key
  webhook_secret VARBINARY(512) NULL,       -- fieldcrypt: x-callback-token Xendit
  webhook_key CHAR(32) NOT NULL,            -- acak, bagian URL webhook (bersama company_id)
  methods VARCHAR(500) NOT NULL DEFAULT '', -- daftar metode yang diaktifkan
  expiry_hours INT NOT NULL DEFAULT 24,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  updated_by BIGINT NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY pay_accounts_webhook (company_id, webhook_key),
  KEY pay_accounts_company (company_id, is_active)
);

CREATE TABLE pay_intents (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  source VARCHAR(100) NOT NULL,             -- "sales.invoice"
  source_id BIGINT NOT NULL,
  reference VARCHAR(100) NOT NULL,          -- nomor dokumen untuk tampilan
  amount DECIMAL(19,2) NOT NULL,
  paid DECIMAL(19,2) NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL,              -- open | partially_paid | paid | overpaid | canceled
  customer_name VARCHAR(200) NOT NULL DEFAULT '',
  customer_email VARCHAR(190) NOT NULL DEFAULT '',
  created_by BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY pay_intents_source (company_id, source, source_id)
);

CREATE TABLE pay_attempts (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  intent_id BIGINT NOT NULL,
  account_id BIGINT NOT NULL,
  order_id VARCHAR(50) NOT NULL,            -- unik di gateway
  provider_ref VARCHAR(100) NOT NULL DEFAULT '',
  url VARCHAR(500) NOT NULL DEFAULT '',
  amount DECIMAL(19,2) NOT NULL,
  fee DECIMAL(19,2) NULL,
  method VARCHAR(30) NOT NULL DEFAULT '',
  status VARCHAR(12) NOT NULL,              -- pending | paid | failed | expired | canceled
  provider_status VARCHAR(30) NOT NULL DEFAULT '',
  expires_at DATETIME(6) NULL,
  paid_at DATETIME(6) NULL,
  checked_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY pay_attempts_order (company_id, order_id),
  KEY pay_attempts_pending (company_id, status, created_at)
);

CREATE TABLE pay_webhooks (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  account_id BIGINT NOT NULL,
  dedupe_key VARCHAR(150) NOT NULL,
  body MEDIUMTEXT NOT NULL,                 -- mentah, untuk penelusuran
  received_at DATETIME(6) NOT NULL,
  processed_at DATETIME(6) NULL,
  error VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY pay_webhooks_dedupe (company_id, account_id, dedupe_key)
);
```

**Tanpa query lintas perusahaan.** URL webhook memuat ID perusahaan
(`/webhooks/{provider}/{companyId}/{webhookKey}`), sehingga akun dicari dengan `company_id` dan
`webhook_key` sekaligus (aturan `scoped-queries`). Kunci acak 32 karakter tetap menjadi rahasia:
tebakan ID perusahaan saja tidak cukup, dan webhook tetap diverifikasi tanda tangannya.

### Kontrak, izin, web

```go
type PaymentClient interface {
	Declare(s Source)
	// Request creates (or reuses) the intent of a source document and a fresh payment link,
	// inside the caller's d.Tx. Amount must be whole rupiah.
	Request(ctx context.Context, r Request) (Link, error)
	// Cancel closes the intent (the document was cancelled); open links expire.
	Cancel(ctx context.Context, source string, sourceID int64) error
	Status(ctx context.Context, source string, sourceID int64) (IntentStatus, error)
}
```

- **Izin:**
  - `payments.read`: daftar dan rekonsiliasi;
  - `payments.manage`: akun gateway, seluruh perusahaan, dicatat audit `security`.
  - Membuat link memakai izin modul pemakai.
- **Web:**
  - halaman "Pengaturan › Pembayaran": akun gateway, URL webhook yang harus didaftarkan di dashboard
    gateway, dan tombol uji koneksi;
  - daftar pembayaran dengan status, metode, dan biaya;
  - laporan selisih rekonsiliasi;
  - status pembayaran dan tombol "Buat link pembayaran" di slot generik `record:status`.

---

## Slot record (cara komponen modul bersama tampil di halaman modul lain)

Slot di kernel berisi komponen statis tanpa props, sedangkan panel lampiran atau status
persetujuan perlu tahu record mana yang sedang dibuka. Agar tidak ada modul yang saling import:

- `ui-core` 0.5.0 menambah konteks generik (tidak tahu domain):
  - `RecordProvider` dan `useRecord()` membawa `{ type, id }` record yang sedang dibuka;
  - `RecordListProvider` dan `useRecordList()` membawa `{ entity }` untuk halaman daftar.
- **Halaman modul pemilik** membungkus slot di halamannya. Contoh di halaman pemasok:
  `<RecordProvider type="suppliers.supplier" id={c.id}>{useSlot("record:status")}…{useSlot("record:activity")}</RecordProvider>`.
- **Modul bersama** mengisi slot dengan komponennya. Komponen itu membaca `useRecord()`, memanggil
  API modulnya sendiri, dan tidak merender apa pun bila jenis record tidak didaftarkan (API
  menjawab 404).
- **Nama slot** `record:status`, `record:activity`, dan `list:actions` didokumentasikan di README
  `ui-core` sebagai milik kit (bukan milik satu modul), karena dipasang oleh banyak halaman.

## Perubahan di modul lain

| Modul | Versi | Perubahan |
|---|---|---|
| `rbac` | 0.7.0 | Kontrak baru `RoleClient.Holders(ctx, permission, branchID) ([]int64, error)`: siapa saja yang memegang izin di cabang itu (termasuk peran seluruh perusahaan). Untuk port `Approvers` di `approvals` (notifikasi penyetuju dan mode `all`) |
| `ui-core` | 0.5.0 | `PrintLayout`, `PrintButton`, CSS cetak; `RecordProvider`/`useRecord`, `RecordListProvider`/`useRecordList`; slot `record:status`, `record:activity`, `list:actions` |
| `ui-gallery` | 0.1.2 | Contoh cetak |
| `customers`, `suppliers` | 0.2.0 | Port terlambat `Importer` (jenis data dengan `Validate`/`Apply`/`Export`) dan `Attachments` (jenis record dengan `Access`). Halaman daftar dan detail membungkus slot dengan `RecordListProvider`/`RecordProvider` |
| `files` | — | Tidak berubah: `Save`, `Open`, `Lock` sudah cukup |
| `mailer`, `settings` | — | Tidak berubah: dipakai lewat kontrak yang ada |

## Pengujian

**Unit** (`kerneltest.Deps`) per modul:
- **`approvals`**: rute dengan syarat; `any`/`all`/`min`; tolak menyetujui sendiri dan lewat
  delegasi; suara kedua 409; `OnDecided` gagal membatalkan keputusan; `Supersede` lalu ajukan ulang;
  kebijakan berubah tidak memengaruhi permintaan yang sedang berjalan; eskalasi.
- **`notifications`**: penggabungan thread; idempotensi; pelaku dilewati; preferensi email;
  `ETag`.
- **`data-import`**: CSV `;` dengan BOM, Windows-1252, `1.234,56`, tanggal serial XLSX, berkas zip
  bom ditolak, mode atomik dan sebagian, sel injeksi pada ekspor, izin saat tugas berjalan.
- **`pdfdoc`**: terbilang (tabel kasus), dokumen 3 halaman dengan judul kolom berulang, enkripsi
  bisa dibuka dengan password yang benar.
- **`fieldcrypt`**: bolak-balik; AD lain gagal; kunci lama tetap terbaca; ciphertext diubah 1 bit
  gagal; indeks buta stabil dan terpotong.
- **`attachments`**: `Access` menolak → 403 di semua rute; record terkunci menolak hapus; tipe
  yang tidak diizinkan ditolak; penyebutan tanpa akses baca tidak diberi tahu.
- **`payments`**: tanda tangan Midtrans benar dan salah; token Xendit salah; webhook ganda diproses
  sekali; webhook "settlement" tetapi Get Status "pending" tidak menandai lunas; webhook datang
  terbalik (settlement lalu pending) tidak mundur; jumlah bersen ditolak; polling menandai
  `expired`.

**Testbed — skenario baru `shared`** (port 18103–18105), satu proyek dengan semua modul ini,
`customers`, `suppliers`, dan fixture `fixinvoice` sebagai pemakai `approvals` dan `payments`:
- Kebijakan "faktur ≥ 10 juta perlu persetujuan kepala cabang". Faktur 5 juta langsung terposting;
  faktur 20 juta menunggu. Pembuat tidak bisa menyetujui (403); kepala cabang menyetujui lalu
  faktur terposting dalam transaksi yang sama. Periode ditutup sebelum disetujui: persetujuan
  ditolak dengan alasan periode, permintaan tetap pending.
- Notifikasi muncul di kotak masuk kepala cabang. Email masuk antrean `mailer`. Jumlah belum dibaca
  dan `ETag` 304.
- Impor 1.000 pelanggan dari XLSX dan CSV `;` berbahasa Indonesia. Pratinjau menunjukkan 3 baris
  salah; mode atomik tidak menulis apa pun; mode sebagian menulis 997. Unduh baris gagal. Ekspor
  dengan sel `=HYPERLINK(...)` keluar ter-escape.
- Lampiran di pemasok. Pengguna tanpa `suppliers.read` mendapat 403 saat mengunduh. Lampiran faktur
  yang sudah diposting tidak bisa dihapus.
- Payments terhadap **gateway tiruan** yang dijalankan testbed: link dibuat; webhook dengan tanda
  tangan salah ditolak; webhook benar dua kali hanya diproses sekali; faktur fixture lunas lewat
  `OnPaid`; perusahaan B tidak melihat pembayaran perusahaan A.
- PDF faktur fixture: dimulai `%PDF`, memuat terbilang; slip terenkripsi tidak bisa dibuka tanpa
  password.

## Risiko yang diverifikasi saat membangun

- **`Access` atas nama orang lain** (untuk penyebutan di komentar). Kernel menyediakan
  `withActor` untuk tugas, tetapi `d.Authz` mungkin hanya menjawab untuk pengguna yang masuk. Bila
  tidak bisa, penyebutan dibatasi ke orang yang sudah terlibat (pembuat, penyetuju, komentator
  sebelumnya).
- **Path modul fpdf di Codeberg** dan kompatibilitas `go mod` dengan host non-GitHub di CI.
  Cadangannya fork di GitHub (`phpdave11/gofpdf`), atau `signintech/gopdf` dengan font yang dimuat
  dari `DATA_DIR`.
- **Advisori keamanan excelize** (GO-2026-6684, GO-2026-6689): pin versi yang sudah memperbaikinya.
- **Polling** 60 detik dikali jumlah pengguna aktif. Indeks `ntf_unread` dan `ETag` diukur di
  testbed. Pindah ke SSE bila terlalu berat.
- **Kunci `FOR UPDATE` di `approvals` bersamaan dengan kunci modul pemilik dalam `OnDecided`**:
  urutan kunci harus permintaan dulu, lalu dokumen. Ini didokumentasikan di README seperti resep
  `docstate`.
- **Midtrans Snap `redirect_url` sebagai link yang dibagikan** selama masa berlakunya. Riset tidak
  menemukan pernyataan resmi; diuji di sandbox.

## Dampak keputusan

| Bila memilih alternatif | Yang berubah |
|---|---|
| K1 event | `approvals` memancarkan `approvals.request.approved/rejected` v1; pemakai memasang `listens`; status dokumen bisa "disetujui, gagal diposting" |
| K3 kunci | Pemakai menolak perubahan selama pending; `Supersede` tidak dipakai |
| K4 boleh | `allow_self` bawaan true |
| K5 template | Kolom `template_key` dan `params`; render saat dibaca |
| K6 SSE | Endpoint stream dan penyebaran antarkontainer (kemungkinan kernel request) |
| K7 Chromium | Kernel request: Chromium di image atau sidecar Gotenberg; template HTML per jenis dokumen |
| K8 kit | Tidak ada modul `data-import`; kit parser dan wizard UI, setiap modul membuat endpoint dan log sendiri |
| K11 env | `pay_accounts` hilang; satu akun lewat env; URL webhook tanpa ID perusahaan |
| K12 satu | Hanya satu implementasi `Provider` di 0.1.0 |

## Urutan kerja

Setiap langkah adalah satu commit (atau beberapa) di branch ini; kit UI dan kit Go lebih dulu,
sesuai `kits-first`.

1. `fieldcrypt` (kit) dan tesnya.
2. `notifications`: dibutuhkan oleh semua modul lain.
3. `rbac` 0.7.0 (`Holders`), lalu `approvals`, dengan fixture `fixinvoice` sebagai pemakai.
4. `data-import`, lalu `customers`/`suppliers` 0.2.0 sebagai pemakai.
5. `attachments`, disambungkan ke `customers`/`suppliers`.
6. `pdfdoc` dan `ui-print` (`ui-core` 0.5.0, `ui-gallery`).
7. `payments` dengan gateway tiruan di testbed.
8. Skenario testbed `shared`, testbed lengkap, katalog, STATUS.md, hasil di rencana ini.

Bila terlalu besar untuk satu PR, potongan alaminya adalah 1–3 (persetujuan dan notifikasi),
4–6 (impor, lampiran, cetak), dan 7 (pembayaran).

## Sumber

**Persetujuan dan notifikasi**
- Frappe Workflow — https://docs.frappe.io/erpnext/user/manual/en/workflows · skema transisi
  (`allow_self_approval`, bawaan 1) —
  https://raw.githubusercontent.com/frappe/frappe/develop/frappe/workflow/doctype/workflow_transition/workflow_transition.json ·
  https://raw.githubusercontent.com/frappe/frappe/develop/frappe/model/workflow.py
- ERPNext Authorization Rule — https://docs.frappe.io/erpnext/user/manual/en/authorization-rule
- Odoo purchase double validation —
  https://raw.githubusercontent.com/odoo/odoo/18.0/addons/purchase/models/purchase_order.py
- SAP release strategy —
  https://help.sap.com/saphelp_46C/helpdata/EN/75/ee14d755c811d189900000e8322d00/content.htm
- Oracle AME voting —
  https://docs.oracle.com/en/cloud/saas/procurement/24c/oapro/various-aspects-of-approval-rules.html
- Talenta approval lembur — https://help-center.talenta.co/hc/en-us/articles/42016410500505
- Power Automate approvals — https://learn.microsoft.com/en-us/power-automate/get-started-approvals
- Pemindai tautan email menyetujui otomatis — https://jira.atlassian.com/browse/JSDCLOUD-16698
- Laravel notifications — https://laravel.com/docs/12.x/notifications · Frappe Notification Log —
  https://raw.githubusercontent.com/frappe/frappe/develop/frappe/desk/doctype/notification_log/notification_log.json
- Knock preferences — https://docs.knock.app/preferences/overview · Novu digest —
  https://docs.novu.co/workflows/digest · GitHub notifications —
  https://docs.github.com/en/rest/activity/notifications

**Impor, PDF, enkripsi**
- excelize — https://pkg.go.dev/github.com/xuri/excelize/v2 ·
  https://raw.githubusercontent.com/qax-os/excelize/master/excelize.go
- OWASP CSV injection — https://owasp.org/www-community/attacks/CSV_Injection
- Frappe Data Import — https://docs.frappe.io/erpnext/user/manual/en/data-import ·
  django-import-export — https://django-import-export.readthedocs.io/en/latest/import_workflow.html
- CLDR simbol angka (pemisah daftar `;` untuk `id`) —
  https://www.unicode.org/cldr/charts/latest/by_type/numbers.symbols.html
- go-pdf/fpdf (pindah ke Codeberg) — https://github.com/go-pdf/fpdf · https://codeberg.org/go-pdf/fpdf
- pdfcpu enkripsi — https://pdfcpu.io/encrypt/encryptpdf
- chromedp — https://github.com/chromedp/chromedp · Gotenberg — https://gotenberg.dev/docs/getting-started/installation
- Go `crypto/cipher` (`NewGCMWithRandomNonce`, batas 2³²) — https://pkg.go.dev/crypto/cipher
- OWASP Cryptographic Storage —
  https://cheatsheetseries.owasp.org/cheatsheets/Cryptographic_Storage_Cheat_Sheet.html
- CipherSweet blind index — https://ciphersweet.paragonie.com/internals/blind-index

**Pembayaran dan lampiran**
- Midtrans notifikasi — https://docs.midtrans.com/docs/https-notification-webhooks · status —
  https://docs.midtrans.com/reference/get-transaction-status · Snap —
  https://docs.midtrans.com/reference/backend-integration · IP —
  https://docs.midtrans.com/docs/ip-address
- Xendit stack terbaru — https://docs.xendit.co/docs/migrating-to-xendit-s-latest-payments-stack ·
  Payment Session — https://docs.xendit.co/apidocs/create-session · keamanan webhook —
  https://docs.xendit.co/docs/integration-security
- Format payroll bank tanpa spesifikasi publik — https://www.bca.co.id/en/Syarat-dan-Ketentuan/KlikBCA-Bisnis ·
  https://www.cimbniaga.co.id/content/dam/cimb/tb/2026/File-Format-Payroll-Bulk-Payment.pdf
- Odoo `ir.attachment` (akses dari induk) —
  https://raw.githubusercontent.com/odoo/odoo/master/odoo/addons/base/models/ir_attachment.py ·
  Frappe File (izin induk, lindungi berkas dokumen yang disubmit) —
  https://raw.githubusercontent.com/frappe/frappe/develop/frappe/core/doctype/file/file.py

**Belum terkonfirmasi** (tidak menjadi dasar keputusan keras):
- path modul fpdf di Codeberg;
- apakah `redirect_url` Snap boleh dibagikan sebagai link berumur panjang;
- kode kanal QRIS di Payments API v3 Xendit;
- dukungan semua glyph yang dibutuhkan pada cp1252;
- apakah Excel Indonesia selalu memakai `;`.
