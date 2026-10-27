# Building a module

From an idea to a released module, for a person or an AI agent with this repository only. This
is the one home of the workflow and the release steps; the rules it follows are in
[AGENTS.md](../AGENTS.md) (each with the check that enforces it), the manifest in
[MODULE_FORMAT.md](MODULE_FORMAT.md), where the library stands in [STATUS.md](../STATUS.md),
and the kernel a module builds on — pinned, read-only — in [`kernel/`](../kernel/README.md).

## 1. Before you start

1. Read [STATUS.md](../STATUS.md): what exists, what is in progress, what is proposed next, the
   known limits. Pick up a proposal from **Next** rather than starting something parallel.
2. Check [CATALOG.md](../CATALOG.md): perhaps a module already does most of it — then it gets a
   new version instead of a sibling.
3. Set up a clone once: `git config core.hooksPath .githooks`. Tools:

   | Tool | For | Install |
   |---|---|---|
   | Node 20+ | every script | — |
   | Go | building, `go test` | any recent Go; the host's `go.mod` picks the toolchain and Go downloads it |
   | `sqlc` 1.31.1 | generating queries | `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1` |
   | `gofmt` | formatting | comes with Go |
   | MySQL 8 + `mysql` client | the testbed's API checks, a sandbox you run | your package manager, or Docker |
   | golang-migrate | `npm run migrate:up` in a sandbox (the testbed installs its own) | `go install -tags mysql github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1` |
   | `air` | `npm run dev:api` in a sandbox (hot reload) | `go install github.com/air-verse/air@latest` |

4. A module that owns tables, needs ports, or holds business rules starts as a **plan**:
   `docs/plans/PLAN-<NAME>.md`, in the shape [docs/plans/README.md](plans/README.md) gives. Open a
   pull request with the plan and an **In progress** line in STATUS.md; the maintainer's review
   of that pull request (`docs/plans/` is code-owned) is the approval. Build after it is merged.

### Working across several commits

On `main`, every commit is a release: the hook checks it against the previous commit, every gate
enforced, and the post-commit hook tags new versions. So build a module **on a branch**:

```bash
git switch -c items
# commit as often as you like: the hook (scripts/check.mjs, also `npm run check`) checks the
# branch against where it left main — one version bump for the whole branch — and
# testbed-covered and status-current only warn; nothing is tagged
git push -u origin items        # CI checks the branch the same way
# open a pull request: its check enforces every gate against main; merging it is the release
```

After the merge, CI tags the new versions on `main` and pushes the tags (the `tags` job). When
you merge locally instead, run `node scripts/tags.mjs` on `main` and `git push --follow-tags` —
`git merge` runs no post-commit hook.

## 2. Decide what it is

| Question | Answer → |
|---|---|
| Shared Go code with no table, route, or wiring (calculations, rules, formats)? | a **Go kit** (`kind: gokit`, `lib/`) — like `money`, `docstate` |
| Generic UI used by several modules? | a **ui kit** addition (`ui-core`, `ui-data`, `ui-forms`, `ui-feedback`) and `ui-gallery` — not a module |
| Owns data and screens? | a **fullstack** module (`api/` + `web/` + `migrations/`) |
| Owns data, no screen (an engine others call)? | a **backend** module — none exists yet; it is a fullstack module without `web/` |
| Screens only, over other modules' APIs? | a **web** module |
| Does its data belong to a company (almost all business data does)? | `"scoped": true` — `company_id` everywhere |
| Needs another module's data or action? | a **port** in your module + a ready-made **adapter** to the provider's `contracts/` — never an import |
| Reacts to something another module did, after it committed? | **`listens`** to its event; publish your own facts with **`emits`** |
| Needs what no module can own (process lifecycle, deploy, request context, the frontend shell)? | a **Kernel request** issue — see AGENTS.md; ship what the current kernel allows meanwhile |

## 3. Start from the closest module

| You need | Look at |
|---|---|
| A CRUD module: list, form, deactivate, permissions | `users` (not scoped) — for company-scoped master data, add what `period`/`numbering` (`"scoped": true`: `company_id` in every table, key, and query) do; `organization`'s branch pages show the list/form/deactivate flow, but it is not a scoped module, so do not copy its queries; the first scoped master-data module becomes the template |
| A company-scoped settings page of its own | `period`, `numbering` |
| A section on the shared Pengaturan › Umum page | the `settings:sections` slot of `settings` (`SettingsPage`) — no module fills it yet |
| Offering a contract other modules wire to | `users` (`UserClient`), `numbering` (`NumberingClient`) |
| A port with a ready-made adapter | `auth` (`UserStore` ← `users`), `numbering` (`Units` ← `organization`) |
| A port to a provider built **after** you (late port) | `users` / `rbac` `Placements` ← `organization` |
| Filling a kernel slot (`d.Auth`, `d.Authz`, `d.Audit`, `d.Scope`) | `auth`, `rbac`, `audit-log`, `organization` |
| Filling another module's UI slot | `organization` (`shell:scope`), `auth` (`sidebar-footer`), `users` (`user-menu:items`) |
| Publishing / hearing events | `organization` emits, `rbac` listens |
| Scheduled work, an outbox | `mailer` (`d.Jobs`) |
| Files and streaming | `files` |
| Posting business documents (number, period lock, cancel, revise) | the fixture `testbed/fixtures/documents/fixinvoice` + the `docstate` README's posting recipe |
| Exact money and quantities | the `money` kit |
| A Go kit | `money`, `docstate` |
| A UI component | `ui-data` (and its page in `ui-gallery`) |

## 4. Build it

### Backend (`api/`)

```txt
api/<id>.module.go     New(d *kernel.Deps, <ports>...) (*Module, error), Routes(mux), Client()
api/contracts/         what you offer: interfaces and plain types (no imports of your module)
api/ports/             what you need, in your own terms (no imports of your module)
api/domain/            types and rules, no I/O
api/application/       use cases; small interfaces for Repository, d.Tx, d.Audit, d.Authz, …
api/infrastructure/    repository on sqlc; queries/*.sql
api/presentation/      HTTP handlers and DTOs
```

- **Permissions:** declare them in `New` with `d.Authz.Declare` *and* in `module.json`
  (`<id>.<action>`). Guard routes with `d.Authz.Require`. Rights that must cover **the whole
  company** (company settings, closing a period) are checked with `d.Authz.Units(ctx, perm)` →
  `AllBranches`; a branch-scoped role passes `Require` but not that.
- **Data:** every query is a sqlc query built per call on `d.Tx.Conn(ctx)`; a use case that
  writes runs in `d.Tx.Do`. `d.Tx` retries the whole function on a deadlock: keep it free of
  side effects outside the transaction (audit and events are inside it — fine).
- **Scope:** read the company with `kernel.ScopeFrom(ctx)`. Background work (jobs, tasks) has no
  scope in a several-companies app — loop `d.Scope.Companies` with `kernel.WithScope`.
- **Errors to the client:** `shared/apperror` — `NotFound`, `Forbidden`, `Conflict` (409, a
  state the user can fix), and `Validation` with `[]validator.FieldError{{Field, Message}}`
  (422; the web reads exactly that shape). Anything else is a 500 and is logged, not shown.
- **Audit:** `d.Audit.Record` for changes people will ask about; `kernel.AuditSecurity` for
  permission changes, reopenings, overrides — with the reason in `Data`.
- **Money and quantities:** the `money` kit (`go_kits`), `DECIMAL` columns (rule
  `exact-numbers`), strings in JSON.

### Web (`web/`)

- `web/module.tsx` exports the `FrontendModule`: routes, `nav`, optional `slots`, `provider`.
- **Navigation:** settings and administration pages go in group `"Pengaturan"` (orders 800–999,
  taken so far: 800 Umum, 810 Penomoran, 820 Tutup buku, 900–980 users, organization, roles,
  outbox, files, audit). Business areas get a group of their own named for the area in
  Indonesian — `"Penjualan"`, `"Pembelian"`, `"Persediaan"`, `"Keuangan"`, `"SDM"` — with orders
  100–699, in steps of 10, set out in the module's plan. `"Pengembang"` (9990+) is for developer
  pages.
- Build pages from the kits (`Page`, `PageHeader`, `SectionCard`, `DataTable`, `Modal`,
  `FormField`, `Notice`, `StatusBadge`, …). Theme tokens only (rule `semantic-colors`), every
  request through `@/shared/services/http-client`.
- `useCan(permission)` answers "in the current scope". When an action needs the right across the
  whole company, let the API say so (a `can_…` flag in the response) instead of guessing.
- Field errors: `apiFieldErrors(err)` reads the 422 list; `apiErrorMessage(err)` the message.

### Language

| What | Language |
|---|---|
| Everything an app user reads: UI copy, API messages, permission descriptions in `module.json`, audit summaries | Indonesian |
| What developers read: code, comments, READMEs, these documents, and in `module.json` the title, summary, when-to-use, and port and env descriptions | English |
| Plans (`docs/plans/`) and commit messages | Indonesian, as so far |

### The rest

- `migrations/NNN_name.up.sql` / `.down.sql` (`001`, `002`, … — the installer renames them to
  timestamps in the project); never edit one once released — add another.
- `adapters/<port>_<provider>.go` (package `modules`): mapping only, declared under the port.
- `README.md`: what it does and does not; how another module uses it (its port, what to call
  when); its permissions and pages; `## Changes` with one line per version.

### Reading kernel/reference in the library

`kernel/reference/` is the skill's guidance for **projects**, pinned as it was. Where it speaks
to a project developer, the library's rules apply instead:

- *A design brief, design research, and the `frontend-design` skill before a new screen* — not in
  the library: a module's look comes from the ui kits and the theme preset (`kits-first`,
  `semantic-colors`). `design-research.md` and `module-library.md` are the skill's, not pinned here.
- *Timestamped migrations made with `migrate:create`* — in the library they are `NNN_…`; the
  installer gives them timestamps.
- *The skill's `scripts/templates/…`* — the same files as a project has them are in
  `kernel/host/` (e.g. `kernel/host/apps/api/internal/kernel/`, `kernel/host/apps/web/src/`).

## 5. Test it

1. **Unit tests** next to the code (`application/service_test.go`) with fakes for the repository
   and `kerneltest.Deps(t)` for the kernel (a `d.Tx` without a database, a clock you move, audit
   and events recorded). Run them in a sandbox: `cd .sandbox/<dir>/apps/api && go test ./...`.
2. **Sandbox — look at every page.** A module alone has no sign-in and no layout, so make one with
   the shell, accounts, and roles:

   ```bash
   npm run sandbox -- app-shell users auth rbac <id> --dir .sandbox/<id>
   mysql -uroot -e "CREATE DATABASE sandbox_mine"          # letters, digits, _ — no dashes
   # in .sandbox/<id>/.env: DB_DSN=root:@tcp(localhost:3306)/sandbox_mine
   #                        USERS_BOOTSTRAP_EMAIL=admin@example.com
   #                        USERS_BOOTSTRAP_PASSWORD=<at least 8 characters>
   cd .sandbox/<id> && npm run migrate:up && npm run dev:api   # and npm run dev:web in another shell
   ```

   Sign in with the bootstrap account (with rbac, the first user becomes Administrator). After changing the
   module in the library, make the sandbox again with `--fresh` (keep `.env` aside first):
   `module.mjs upgrade` cannot merge a module that has never been committed.
3. **Testbed scenario** (required — rule `testbed-covered`), described below.
4. Run `TESTBED_DB_DSN='root:@tcp(localhost:3306)/' npm run testbed -- --only <scenario>` while
   iterating, the full `npm run testbed` before committing. Its last line —
   `All testbed checks passed: N checks.` — is what STATUS.md › Last verified records. Without
   `TESTBED_DB_DSN` the API checks are skipped and it says so: that is not a full run.

### Writing a testbed scenario

`scripts/testbed.mjs` holds one `async function` per scenario, listed in `const scenarios = {…}`
at its end and described in its header comment (rule `testbed-documented` — STATUS.md lists the
scenarios from there). A new business module usually **extends `documents`** (it has a library
copy, an admin, organization, period, and numbering ready); a module with a different
neighbourhood gets its own scenario. Helpers:

| Helper | Does |
|---|---|
| `scaffold(name)` | a fresh project from the pinned host in `.testbed/<name>-*`; returns its folder |
| `copyLibrary(lib)` | a working copy of the library (for fixtures, or changing it in the test) |
| `installModule(lib, dir, id, { ports })` | what `module.mjs add` does — kits first, ports by provider id or `{ expr }` |
| `build(dir)` | npm install, go mod tidy, sqlc, gofmt, vet, `go test`, build web and API — `false` on failure |
| `parseDsn(DB_DSN)`, `freshDatabase(dir, db, name)` | a throwaway database, migrated; returns its DSN |
| `mysqlQuery(db, sql, name)` | one value from SQL — check what was stored |
| `startApi(dir, port, env)` / `stopApi(api)` | run the built API; `api.output()` is its log |
| `client(port).call(method, path, body, headers)` | HTTP with a cookie jar; `{ status, body }` |
| `expect(label, ok, detail)`, `skip(why)`, `waitFor(fn, ms)` | a check; a reported skip; wait for an async condition |

The shape, from `documents`:

```js
async function myarea() {
  log("\n=== myarea: … ===");
  const lib = mkdtempSync(join(LIB, ".testbed", "libmy-"));
  const dir = scaffold("myarea");
  let api;
  try {
    copyLibrary(lib);                      // + cpSync of fixtures into lib/modules, git init/commit
    for (const [id, opts] of [["app-shell"], ["users"], ["auth", { ports: { UserStore: "users" } }], ["myarea"]])
      installModule(lib, dir, id, opts);
    if (!build(dir)) return;
    if (!DB_DSN) return skip("myarea: API checks against MySQL — set TESTBED_DB_DSN");
    const db = parseDsn(DB_DSN);
    const name = `elco_testbed_my_${Date.now()}`;
    try {
      const dsn = freshDatabase(dir, db, name);
      api = await startApi(dir, 18103, { DB_DSN: dsn, USERS_BOOTSTRAP_EMAIL: "admin@example.com", USERS_BOOTSTRAP_PASSWORD: "admin-pass-123" });
      const admin = client(18103);
      await admin.call("POST", "/api/v1/auth/login", { email: "admin@example.com", password: "admin-pass-123" });
      // allow, refuse (401/403/409/422), isolate companies — then check what MySQL holds
    } finally {
      stopApi(api);
      try { mysql(db, `DROP DATABASE IF EXISTS ${name};`); } catch {}
    }
  } catch (err) {
    fail(`myarea: ${err.message}`);
  } finally {
    stopApi(api);
    if (!KEEP) { rmSync(dir, { recursive: true, force: true }); rmSync(lib, { recursive: true, force: true }); }
  }
}
```

**Ports:** every scenario uses its own API ports. Taken: 18091–18099 and 18101–18102 (some are
held in variables — `grep -n "1809[0-9]\|181[0-9][0-9]" scripts/testbed.mjs` finds them all);
take the next free ones, 18103 onward, and add them to this line.
`scripts/` is code-owned: a new or changed scenario is reviewed by the maintainer with the
module's pull request.

## 6. Pitfalls already paid for

- **sqlc's MySQL parser** rejects some column and argument names that MySQL accepts: `last_value`
  and `until` are keywords there. Rename them (`last_number`, `due`).
- **sqlc types:** `DECIMAL` is a `string` (`sql.NullString` when nullable); `COALESCE` over a
  `LEFT JOIN` becomes `interface{}` — split into inner joins and merge in Go; one named argument
  cannot have two types.
- **MySQL `:execrows`** counts *changed* rows: an UPDATE that sets the same values reports 0.
  Do not read 0 as "not found" unless the row must change (a guarded status change does).
- **`DATE` columns** arrive as midnight UTC (the connection runs in UTC): convert with
  `kernel.DateOf(t, time.UTC)`; business "today" is `kernel.Today(ctx, d.Clock)`.
- **JSON decoding:** `request.DecodeJSON` turns a type error into 400 "Kolom … berisi tipe data
  yang salah". A money value that does not parse is such an error; for a message per rule, take a
  string and parse it yourself.
- **Locks:** a duplicate `INSERT IGNORE` takes a *shared* lock on the existing row;
  `INSERT … ON DUPLICATE KEY UPDATE` an *exclusive* one. `period` uses the first for postings and
  the second for lock changes. A waiting exclusive lock queues new shared ones behind it.
- **Late ports:** a provider that needs your module (organization needs users) cannot be passed
  to your `New`; take it through a setter wired in `register.go.late`.
- **Events:** name them with a string literal and the declared version at the `Publish` call, or
  the rule cannot check them.
- **Known limits** of the library and kernel (a flaky kernel test under load, best-effort audit,
  …) are listed in STATUS.md — read them before blaming your module.

## 7. Release

The one list of release steps. On a branch, steps 1–5 happen before the pull request is merged;
the merge is the release.

1. Raise the version (patch: fix, minor: feature, major: breaking) and add the `## Changes` line.
2. Make sure a testbed scenario installs and exercises the module (`testbed-covered`).
3. Run the full testbed with MySQL (§ 5).
4. `npm run catalog` — regenerates `CATALOG.md`, `catalog.json`, and STATUS.md's snapshot. Then
   update **STATUS.md** by hand: a new Log line naming each released module with its version (the
   check looks for it among the lines you add), Last verified (the run from step 3), your line out of In progress, and
   Next and Known limits if they moved. Update your plan's results.
5. `npm run check` (branch-aware, like the hook), then commit — everything the check reads must
   be staged (the hook refuses unstaged or untracked changes in `modules/`, `scripts/`,
   STATUS.md, … so it approves what the commit takes).
6. On `main` — a pull request merged, or a commit made there — the version is tagged
   `<id>/v<version>`: by CI after a merge on GitHub, by the post-commit hook for a local commit
   (`node scripts/tags.mjs` after a local merge); `git push --follow-tags`.
7. Changes to code-owned paths (`.github/CODEOWNERS` lists them) need the maintainer's review on
   the pull request.

## Done means

- [ ] `module.json` complete; `README.md` with usage and `## Changes`
- [ ] Unit tests; a testbed scenario installs it and exercises allow, refuse, and isolation
- [ ] Every page looked at in a sandbox
- [ ] `npm run check` and the full testbed pass
- [ ] STATUS.md (Log with the version, Last verified, In progress) and the plan updated
- [ ] Merged to `main`, tagged, pushed with `--follow-tags`
