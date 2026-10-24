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
node $ELCO_MODULES/scripts/module.mjs detach auth                # the project owns it from now on
node $ELCO_MODULES/scripts/module.mjs promote invoice --migration <file>   # project module → library
```

A port is wired with `--port <Port>=<provider module>` (its ready-made adapter is used) or
`--port <Port>=expr:<Go expression>` for an adapter the project writes in
`apps/api/internal/modules/`. `add` refuses — and changes nothing — when a required port is not
wired, a ui kit it builds on is missing, or the kernel version differs.

**`modules.lock.json`** (commit it) records per module: version, the library commit it came
from, port wiring, the hash of every file as the library rendered it, its migrations, and its
registry lines. `status` compares the project's files with those hashes; `upgrade` re-renders
the module at the recorded commit (base) and at the library's current tree (theirs) and merges
them into the project's files with `git merge-file`, so the project's changes survive and only
real overlaps conflict. Migrations are forward-only: new ones are added with fresh versions, an
applied one is never rewritten. Install from a committed library so upgrades have an exact base.

Customize an installed module in new files of its own where possible — fewer conflicts later.
When a module has diverged for good, `detach` it.

## Layout

```txt
modules/<id>/        module.json, README.md, and the folders it installs (api/, web/, shared/,
                     migrations/, adapters/)
themes/<id>/         preset.json, theme.css, fonts.ts, DESIGN.md
scripts/catalog.mjs  validates manifests (incl. "no module imports another") → CATALOG.md, catalog.json
scripts/testbed.mjs  installs modules into freshly scaffolded projects and tests them end to end
scripts/module.mjs   the CLI projects use (add, status, upgrade, detach, promote, …)
scripts/lib/         install/upgrade core, library reader (working tree or a commit), promote
testbed/fixtures/    project-side code the testbed needs (e.g. a project-owned UserStore)
```

## Working on the library

```bash
npm run catalog                 # after changing any module.json / preset.json
npm run check                   # manifests valid + catalog current (CI runs this)
TESTBED_DB_DSN='root:@tcp(localhost:3306)/' npm run testbed -- --skill ~/path/to/monorepo-elcodelabs
```

The testbed needs Go, `sqlc`, `gofmt`, Node, and — for the API checks of the full scenario —
a MySQL server and the `mysql` client (it creates and drops a throwaway database). Scenarios:

| Scenario | Proves |
|---|---|
| `full` | preset + ui-core + app-shell + users + auth (UserStore ← users) build, pass unit tests, and work end to end: sign-in, users CRUD, deactivation ending a session, logout, SPA served |
| `auth-alone` | auth runs on a UserStore the project wrote itself — no users module |
| `users-alone` | users runs without any auth provider; its guarded routes refuse (401) |
| `lifecycle` | the CLI on a throwaway git copy of the library: add (and a refused add), status, upgrade merging the project's and the library's changes (then the project still builds), a conflicting upgrade, detach, promote (and a refused promote) |

A new module is done when its README and `module.json` are complete, the catalog check passes,
and a testbed scenario covers it.

## Versions

Each module has its own version, tagged `<id>/v<version>` (e.g. `auth/v0.1.0`). A module states
the kernel version it is built against (`requires_kernel`).
