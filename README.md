# elcodelabs-modules

Reusable modules, UI kits, and theme presets for projects scaffolded with the
`monorepo-elcodelabs` skill (Go modular-monolith API + React 19 / shadcn web). Instead of writing
the same features again, a project **installs** what fits and **builds** only what is custom.

**Start at [CATALOG.md](CATALOG.md)** — the generated list of everything here, with when to use
each piece and what it needs.

## How it fits together

- A module is **copied into the project** (not imported as a package), so the project owns the
  code and can adapt it. Installing writes only into the module's own folders and between the
  project's `elco:*` markers (`registry.go`, `sqlc.yaml`, `app/modules.ts`).
- **Modules never import each other.** A module offers `contracts/` and states what it needs as
  `ports/`; the project's composition root connects them with small adapters. Ready-made
  adapters ship with the module that needs them. Authentication is the kernel's `d.Auth` slot,
  filled by the `auth` module.
- **Look comes from the theme**: modules use only semantic token classes, so they fit any preset
  — or none.
- **Generic UI comes from four kits** — `ui-core` (primitives, page vocabulary, status,
  states, values, modal/sheet), `ui-data` (DataTable, pagination, toolbar, filters, menus),
  `ui-forms` (FormField, search selects, line items, dropzone, stepper), `ui-feedback` (toasts,
  global loader, confirm/reason dialogs, error boundary). They carry the Bingkai Navy component
  design, default to Indonesian copy, and take every visible string as a prop. The `ui-gallery`
  module renders all of them on one page (`/ui-gallery`). A module lists the kits it builds on
  in `ui`; `add` and `upgrade` install missing ones first.

Format and rules: [docs/MODULE_FORMAT.md](docs/MODULE_FORMAT.md).

## Using it in a project

From the project root (`$ELCO_MODULES` is where this library is cloned):

```bash
node $ELCO_MODULES/scripts/module.mjs list                       # what exists (✓ = installed)
node $ELCO_MODULES/scripts/module.mjs info auth                  # README, ports, env
node $ELCO_MODULES/scripts/module.mjs preset bingkai-navy
node $ELCO_MODULES/scripts/module.mjs add ui-core app-shell users
node $ELCO_MODULES/scripts/module.mjs add auth --port UserStore=users
node $ELCO_MODULES/scripts/module.mjs status                     # versions, files changed here
node $ELCO_MODULES/scripts/module.mjs upgrade users              # 3-way merge; exit 2 = conflicts
node $ELCO_MODULES/scripts/module.mjs remove ui-gallery          # uninstall (tables and data stay)
node $ELCO_MODULES/scripts/module.mjs detach auth                # the project owns it from now on
node $ELCO_MODULES/scripts/module.mjs promote invoice --migration <file>   # project module → library
```

A port is wired with `--port <Port>=<provider module>` (its ready-made adapter is used) or
`--port <Port>=expr:<Go expression>` for an adapter the project writes in
`apps/api/internal/modules/`. `add` installs the ui kits a module lists that the project lacks, and
refuses — changing nothing — when a required port is not wired or the project's kernel is older
than the module's `requires_kernel`.

**`modules.lock.json`** (commit it) records per module: version, the library commit it came
from, port wiring, the hash of every file as the library rendered it, its migrations, and its
registry lines. `status` compares the project's files with those hashes; `upgrade` re-renders
the module at the recorded commit (base) and at the library's current tree (theirs) and merges
them into the project's files with `git merge-file`, so the project's changes survive and only
real overlaps conflict. Migrations are forward-only: new ones are added with fresh versions, an
applied one is never rewritten. Install from a committed library so upgrades have an exact base.

Customize an installed module in new files of its own where possible — fewer conflicts later.
When a module has diverged for good, `detach` it.

## Changing the library

Read [AGENTS.md](AGENTS.md) first — the architecture is locked, and it lists every rule with
the check that enforces it. After cloning, enable the pre-commit hook once:
`git config core.hooksPath .githooks`.

This repository is self-contained: the kernel modules build on is pinned in
[`kernel/`](kernel/README.md) (a host project from a released monorepo-elcodelabs, its kernel
references, and a lock of every file's hash). You do not need the skill to develop, try, or
test a module.

## Layout

```txt
modules/<id>/        module.json, README.md, and the folders it installs (api/, web/, shared/,
                     migrations/, adapters/)
themes/<id>/         preset.json, theme.css, fonts.ts, DESIGN.md
scripts/catalog.mjs  validates manifests and the architecture rules (scripts/lib/rules.mjs) → CATALOG.md, catalog.json
kernel/              the pinned kernel: host/ (a scaffolded project), reference/, kernel.lock.json — read-only
scripts/testbed.mjs  installs modules into fresh copies of the pinned host and tests them end to end
scripts/sandbox.mjs  a project from the pinned host with the modules you name, to develop and try them
scripts/kernel-sync.mjs  the maintainer's tool to pin a released kernel (kernel/ and its catalog, own commit)
scripts/module.mjs   the CLI projects use (add, status, upgrade, detach, promote, …)
scripts/lib/         install/upgrade core, library reader (working tree or a commit), promote
testbed/fixtures/    project-side code the testbed needs (e.g. a project-owned UserStore)
```

## Working on the library

```bash
npm run catalog                 # after changing any module.json / preset.json
npm run check                   # manifests valid + catalog current + kernel intact (CI runs this)
npm run sandbox -- <id>...      # .sandbox/<ids>: the pinned host + your modules, ready to run
TESTBED_DB_DSN='root:@tcp(localhost:3306)/' npm run testbed
```

The testbed needs Go, `sqlc`, `gofmt`, Node, and — for the API checks of `full` and `no-rbac` —
a MySQL server and the `mysql` client (it creates and drops throwaway databases; migrations run
through the project's `npm run migrate:up` with golang-migrate, installed into `.testbed/bin`).
Scenarios:

| Scenario | Proves |
|---|---|
| `full` | preset + the four ui kits + every module, all ports wired, build, pass unit tests, migrate with golang-migrate, and work end to end against MySQL and an SMTP sink the testbed runs: sign-in and its limits, users and own profile, sessions ended by sign-out / deactivation / password change, cross-site and non-JSON writes refused, roles and permissions (no escalation, administrators protected), change password, forgot password by email (one-time link from `APP_URL`), the outbox, settings and a logo streamed into files (SVG and disguised HTML refused), the audit trail (actors named, no secrets), and two API processes delivering each email exactly once |
| `no-rbac` | users + auth without rbac: the kernel's default — any signed-in user passes `users.manage` |
| `auth-alone` | auth runs on a UserStore the project wrote itself — no users module, no mailer: password features report themselves off and the optional ports are `nil` |
| `users-alone` | users runs without any auth provider; its guarded routes refuse (401) |
| `lifecycle` | the CLI on a throwaway git copy of the library: add (and a refused add), status, upgrade merging the project's and the library's changes (then the project still builds), a conflicting upgrade, detach, wiring a port later (the consumer moves after its provider in the registry), remove, release-tag fallback, promote (refused when the module imports another or breaks an architecture rule), and the pinned kernel: an edit to it, a module needing a newer kernel, and a kernel update mixed with a module change are refused |
| `kernel6` | fixture modules (`testbed/fixtures/kernel6`: a Go kit, two scoped modules joined by a port, a stand-in for auth and organization) against MySQL: an order and its stock in one `d.Tx` across modules (a refused order leaves nothing), an event delivered once across two containers, companies kept apart and an unplaced scope refused, a missed calendar tick caught up once; and the rules refusing a query without `company_id`, a module opening its own transaction, and an undeclared event |

A new module is done when its README and `module.json` are complete, the catalog check passes,
and a testbed scenario covers it.

## Versions

Each module has its own version, tagged `<id>/v<version>` (e.g. `auth/v0.4.0`) by the post-commit
hook — push with `git push --follow-tags`. A project records the commit it installed from and
falls back to the tag when that commit is gone. A module states the minimum kernel version it
needs (`requires_kernel`), at most the pinned kernel in `kernel/kernel.lock.json`.

The kernel itself is versioned in monorepo-elcodelabs and released as tags
`monorepo-elcodelabs/kernel/v<N>` there. The maintainer pins one here with
`npm run kernel:sync -- --skill <monorepo-elcodelabs folder>` after the testbed passes on it
(`npm run testbed -- --skill <folder>` tries an unreleased kernel first). A module developer
who needs more from the kernel opens a "Kernel request" issue — see AGENTS.md.
