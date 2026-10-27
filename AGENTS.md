# Working on elcodelabs-modules

This file is for anyone who changes this library — a person or an AI agent. Read it before
adding or changing a module. The architecture here is **locked**: it is the architecture of
the `monorepo-elcodelabs` skill, and every module must install into a project that skill
scaffolded and fit there as if it had been written in that project.

**Start with [STATUS.md](STATUS.md)** — where the library stands — and build with
[docs/BUILDING_MODULES.md](docs/BUILDING_MODULES.md). Whatever you change, leave STATUS.md true:
the checks below refuse a release that does not update it.

**This repository is all you need.** The kernel modules build on is pinned in
[`kernel/`](kernel/README.md): a project scaffolded by a released skill (`kernel/host/`), the
skill's references for the kernel contract (`kernel/reference/`), and a lock with every file's
hash. The testbed and the sandbox start from it; nothing reads the skill (why, and on what
grounds: [docs/decisions/0001-pinned-kernel.md](docs/decisions/0001-pinned-kernel.md)).

## Where the architecture is defined

The skill owns it; its references come pinned with the kernel, read-only:

| Topic | Read |
|---|---|
| Module boundaries, ports, contracts, composition root | `kernel/reference/backend-modular-monolith.md` |
| Go kernel (`d.Auth`, `d.Authz`, `d.Jobs`, `d.Audit`, `d.Config`, versions), module contract, registry | `kernel/reference/backend-go.md`, and the kernel's own code with its doc comments: `kernel/host/apps/api/internal/kernel/`, `…/internal/shared/` |
| Frontend structure, `FrontendModule`, slots, shadcn/ui, tokens | `kernel/reference/frontend.md`, `kernel/host/apps/web/src/shared/` |
| `/api/v1` conventions | `kernel/reference/api-standard.md` |
| The project a module lands in | `kernel/host/` (its `CLAUDE.md`, `.claude/rules/`, and `knowledge/` are what that project's developers read) |

If a module needs something the kernel does not offer, see
[When a module needs more from the kernel](#when-a-module-needs-more-from-the-kernel) — never a
module that works around it, and never an edit under `kernel/`.

## Rules

`node scripts/catalog.mjs --check` enforces every rule marked ✓ and names the rule in its
message (`rules-documented` keeps this table complete). `npm run check`, the pre-commit hook,
and CI run it with `--since`, which adds the history rules — through `scripts/check.mjs`, which
on a work branch checks the branch as one change against `main` with `testbed-covered` and
`status-current` only warning; a pull request's check and every commit to `main` enforce them.

| Rule | What it means | Checked |
|---|---|---|
| `no-module-imports` | A module never imports another module — not its internals, not its `contracts/`, in Go or in the web (`@/modules/<other>`, or a relative path out of its folder). What it needs is a port in its own `ports/`; what it offers is its `contracts/`. | ✓ |
| `adapters-declared` | An adapter (`adapters/*.go`, package `modules`) joins this module's `ports` to one provider's `contracts` and is declared under that port in `module.json`. Mapping only, no logic. | ✓ |
| `kernel-only` | Go code builds on the kernel (`internal/kernel`), `internal/config`, and `internal/shared/*` — nothing else of the project. | ✓ |
| `go-module-contract` | `api/<id>.module.go` holds `New(d, ports...) (*Module, error)`, `Routes(mux)`, and `Client()` when it offers a contract — and `Use<Port>(…)` for each late port (`register.go.late`). | ✓ (file) · testbed (shape) |
| `go-layers` | `api/` holds only `contracts/ ports/ application/ domain/ infrastructure/ presentation/`. Domain, contracts, and ports import nothing of the module; application never imports infrastructure or presentation; presentation reaches storage only through application; only infrastructure touches `database/sql`. | ✓ |
| `sqlc-not-orm` | Data access is `database/sql` + sqlc (`infrastructure/queries`). No ORM. | ✓ |
| `declared-go-deps` | Every third-party Go import is in `go_dependencies` (module path → the version it was tested with), except the kernel's own (`go-sql-driver/mysql`, `joho/godotenv`). The installer pins them in the project's `go.mod`. | ✓ |
| `config-through-kernel` | Settings come from `config.Get` / `config.Require`, never `os.Getenv`. A required one fails `New`. | ✓ |
| `auth-through-kernel` | Guard handlers with `d.Auth.Require` / `d.Authz.Require("<id>.<action>")`, declare permissions with `d.Authz.Declare` in `New` and in `module.json`, hide UI with `useCan`. Never a port for auth. | names ✓ · testbed |
| `own-tables-only` | Migrations and `infrastructure/queries` create, alter, read, and write only tables in `owns_tables`, and no table is owned by two modules. No foreign key or join to another module's table — relations are primitive IDs; ask the owner through a port. Data other modules point at is deactivated, never deleted. | ✓ (tables) · review (deactivate) |
| `exact-numbers` | Migrations create no `FLOAT`, `DOUBLE`, or `REAL` column: money and quantities are `DECIMAL` (`DECIMAL(19,2)` money, `DECIMAL(19,4)` quantities, `DECIMAL(19,10)` rates — the `money` kit). A column that truly needs a float (map coordinates) ends its line with `-- float: <why>`. | ✓ |
| `migrations` | `NNN_snake_case.up.sql` + `.down.sql`, numbered 001, 002, … without gaps. Only fullstack and backend modules have them. | ✓ |
| `manifest` | `module.json` / `preset.json` follow [docs/MODULE_FORMAT.md](docs/MODULE_FORMAT.md): required fields, id = folder, semver, a known kind, kits that exist, adapters declared with their files, registration matching the kind. | ✓ |
| `module-layout` | A module holds only `module.json`, `README.md`, and `api/ web/ shared/ lib/ migrations/ adapters/`, as its kind allows (fullstack: api+web; backend: api; web: web, optionally shared; ui: shared, optionally a web provider; gokit: lib). | ✓ |
| `install-targets` | `api` → `apps/api/internal/modules/<id>`, `web` → `apps/web/src/modules/<id>`, `shared` → `apps/web/src/shared`, `lib` → `apps/api/internal/shared/<id>`. Nothing else is written into a project except between the `elco:*` markers. | ✓ |
| `late-ports` | A `register.go.late` line wires an optional, declared port that is not also wired in the module's block (see MODULE_FORMAT › Placeholders). | ✓ |
| `one-owner-per-file` | No two modules install the same file. | ✓ |
| `text-only` | Every file in a module is UTF-8 text — hashes, 3-way merges, and review depend on it. Images and fonts belong in a preset or the project. | ✓ |
| `domain-agnostic-shared` | Code installed into `shared/` never imports a module and knows no domain (no order statuses, no user roles). | ✓ (imports) · review |
| `declared-kits` | A module that imports a kit's file lists that kit in `ui`; only `ui` kits are shareable; kits do not form a cycle. | ✓ |
| `frontend-registration` | The web side contributes only through its `FrontendModule` (`web/module.tsx`): routes, `nav`, `slots`, and at most a `guard`, `layout`, or `provider`. | ✓ (entry) · testbed |
| `semantic-colors` | Theme token classes only (`bg-primary`, `text-muted-foreground`, …). No palette classes, hex, `rgb()`, or arbitrary color values — a module must look right under any preset. | ✓ |
| `one-http-client` | Every request goes through `@/shared/services/http-client`. No `fetch`, no new axios instance. | ✓ |
| `kits-first` | Generic UI comes from the kits (`ui-core`, `ui-data`, `ui-forms`, `ui-feedback`; see them all in `ui-gallery`). A missing generic piece is added to a kit, not written inside a module. Visible text defaults to Indonesian and is a prop. | review |
| `version-bump` | Any change to what a module installs raises its version (patch: fix, minor: feature, major: breaking) with a line under `## Changes` in its README. | ✓ `--since` |
| `forward-only-migrations` | A released migration is never edited or removed — projects have already applied it. Add a new one. | ✓ `--since` |
| `released-is-forever` | A released module or preset is never deleted — projects upgrade from it. Deprecate it in its README instead. | ✓ `--since` |
| `release-tags` | Every released version has the tag `<id>/v<version>` once it is on `main` (the post-commit hook creates it for a commit made on `main` — push with `git push --follow-tags`; CI's `tags` job creates and pushes it after a merge on GitHub). A project whose recorded commit is gone upgrades from the tag. | ✓ CI on push to `main` |
| `pinned-kernel` | `kernel/` is exactly what `kernel/kernel.lock.json` records — no file edited, added, or removed. It changes only through `scripts/kernel-sync.mjs`. | ✓ |
| `kernel-pinned` | A module's `requires_kernel` is at most the pinned kernel: a module builds only on a kernel it can be tested against. | ✓ |
| `kernel-alone` | A commit that changes `kernel/` changes nothing else but the catalog it regenerates (`CATALOG.md`, `catalog.json`, and STATUS.md, which name the pinned kernel — `kernel-sync.mjs` also writes STATUS.md's log line), so a kernel update is reviewed and reverted on its own. | ✓ `--since` |
| `adapter-requires` | An adapter's `requires` (oldest provider version it compiles against) is a released version of that provider. | ✓ |
| `declared-go-kits` | A module (or kit) that imports a Go kit (`internal/shared/<kit>`) lists it in `go_kits`. | ✓ |
| `gokit-pure` | A Go kit imports only the standard library, its `go_dependencies`, and other Go kits — no kernel, config, module, or environment; no tables, no wiring; never named like a kernel `shared/` package. | ✓ |
| `tx-aware` | A module on kernel 6 builds its queries per call on `d.Tx.Conn(ctx)`, never opens a transaction itself, never uses the kernel's `DB`, and runs no SQL except through sqlc or on `d.Tx.Conn(ctx)` (or a variable holding it) — so a caller's `d.Tx` spans it. | ✓ |
| `scoped-tables` / `scoped-queries` | A `"scoped": true` module has `company_id` in every table and in every query — in the statement's own `WHERE` (not only a subquery's), in an INSERT's columns and in what an INSERT … SELECT copies. Its primary and unique keys include `company_id` (a surrogate `id` aside), so an upsert never reaches another company's row. A system query spanning every company says `-- all companies` and is named `System…`. | ✓ |
| `kernel-tables` | Tables named `kernel_*` belong to the kernel; no module owns them (and `own-tables-only` keeps them out of its queries). | ✓ |
| `testbed-covered` | Every module is installed by at least one testbed scenario (directly, or as a kit of one that is). | ✓ |
| `testbed-documented` | Every testbed scenario has a line in the header comment of `scripts/testbed.mjs` — STATUS.md lists the scenarios from there. | ✓ |
| `rules-documented` | Every rule a check reports is in this table. | ✓ |
| `status-current` | A change that releases a module or preset, moves the kernel, or changes the rules or the testbed's scenarios also updates the hand-written part of STATUS.md (Log; Last verified, In progress, Next, Known limits where they moved), and names each released module or preset with its version on a line it adds. Its snapshot is generated and must be current. | ✓ (`--since` for the hand-written part) |
| `events-declared` | A module publishes only events it declares in `emits` (named `<id>.<thing>.<past-verb>`, versioned, and written at every `Publish(ctx, name, version, …)` — through whatever holds `d.Events` — as a string literal with the declared version); it subscribes only through `listens` (wired in the composition root), to events some module emits. | ✓ |

The Go and SQL rules read the code as written — plainly (`d.Tx.Conn(ctx)`, `d.Events.Publish(ctx,
"…", 1, …)`, `WHERE company_id = ? AND …`). Code bent to slip past them (reflection, an `OR`
beside the company filter, a helper that hands out the pool) is caught in review, not here.

## Adding or changing a module

The whole path — decide, plan, build, test, release — is
[docs/BUILDING_MODULES.md](docs/BUILDING_MODULES.md); its § 7 Release is the one list of release
steps. In short: start from the closest module, build the UI from the kits (a missing generic
piece goes into a kit and `ui-gallery`, in its own change), cover it in a testbed scenario, raise
the version with a `## Changes` line, `npm run catalog`, update STATUS.md, run the check and the
full testbed, commit (the post-commit hook tags releases on `main`), and
`git push --follow-tags`.

Never bypass the hook (`git commit --no-verify`) to land a rule violation. If a rule is wrong,
change the rule in `scripts/lib/rules.mjs` and this table in the same commit, and say why.
`kernel/`, `scripts/`, the CI, these documents, and the plans have code owners
(`.github/CODEOWNERS`, the maintainer @DimasPrasetio): a change to them — and so a new testbed
scenario, and a plan's approval — needs the maintainer's review on the pull request.

## When a module needs more from the kernel

1. **Build it in the module first.** Most needs fit there: a table, a job (`d.Jobs.Every`), an
   audit record (`d.Audit.Record`), settings (`config.Require`), a file (`d.Config.DataDir`).
2. **Need another module? Use a port** in your module, with a ready-made adapter to the provider's
   `contracts/` — never an import.
3. **Still not possible?** Only these are the kernel's: the process lifecycle (`cmd/server`),
   deploy and infrastructure (Dockerfile, compose, `deploy.sh`, the server), the request context,
   and the frontend shell contract. If the need is one of them **and** several modules need it,
   open a **Kernel request** issue (template in `.github/ISSUE_TEMPLATE/`). Meanwhile the module
   ships what the current kernel allows.
4. The skill's maintainer decides, releases the kernel in monorepo-elcodelabs (after running this
   testbed against it), and brings it here with `npm run kernel:sync -- --skill <path>`: one
   commit that touches only `kernel/`, the regenerated catalog, and STATUS.md. Modules may then raise
   `requires_kernel`.

A kernel version only adds: modules released for kernel N keep compiling and behaving the same
on N+1. Something is removed only after a kernel that marks it deprecated, in a kernel the skill
declares breaking, with upgrade notes (`kernel/reference/backend-go.md` › Kernel versions).

## One-time setup in a clone

```bash
git config core.hooksPath .githooks   # pre-commit: the rules; post-commit: release tags
```
