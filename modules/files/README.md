# files

Stores uploaded files for other modules. Fullstack (Go + React), owns `files`.

**How a module uses it.** It declares its own port (e.g. settings' `FileStore`) and an adapter in
the composition root maps it to `contracts.FileClient` — `Save`, `Get`, `Open`, `Delete`. The
module serves its files through **its own endpoint**, deciding who may see them; files has no
public URL of its own.

**Storing.** `Save` reads the first 512 bytes and decides the type from the **content**, not the
name — a `.png` holding HTML is HTML, and refused. Allowed by default: PNG, JPEG, WebP, GIF, PDF,
Word/Excel/PowerPoint (.docx/.xlsx/.pptx), plain text and CSV. **Never SVG or HTML** (script in the
app's origin). A module narrows the types and the size for its own uploads (`Upload.Allowed`,
`MaxBytes`). The bytes stream to `DATA_DIR/files/<yyyy>/<mm>/<random>` — never into memory or
`/tmp`, which is RAM in the container — through a `.part` file renamed on completion; size and
SHA-256 are computed on the way. A refused upload leaves nothing behind. Names are cleaned (no
path, no control characters).

**Deleting** takes a file out of use at once; its bytes stay 30 days (recoverable from disk and
backups), then a daily job (`d.Jobs`) removes them, and abandoned partial writes.

**Server.** `DATA_DIR` is the volume `~/<slug>/data` (deploy.sh hands it to uid 10001); the
server's nightly backup snapshots it with the database. The API does not start if the folder is
not writable. The server's read and write timeouts (30 s / 60 s) are sized for API calls: the
download route gives itself time for the file's size, and a module that accepts uploads or
serves its own files does the same — `request.AllowTransfer(w, maxBytes)` (kernel 5) at the top
of the handler.

**Kernel 5:** `Config.DataDir`, `d.Jobs`, `d.Audit` (administrator deletions), `request.AllowTransfer`.

**Web:** `/files` — "Berkas": every stored file with its module, type, size; download (as an
attachment — never rendered in the app) and delete (`files.manage`). Nav entry in "Pengaturan"
(`files.read`).

**Not in this version:** thumbnails or previews, object storage/CDN, quotas, virus scanning.

## Changes

- **0.1.0** — first version.
