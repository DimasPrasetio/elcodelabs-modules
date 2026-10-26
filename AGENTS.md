# Working on elcodelabs-modules

This file is for anyone who changes this library — a person or an AI agent. Read it before
adding or changing a module. The architecture here is **locked**: it is the architecture of
the `monorepo-elcodelabs` skill, and every module must install into a project that skill
scaffolded and fit there as if it had been written in that project.

## Where the architecture is defined

Not here. The skill owns it, and this library follows it:

| Topic | Source of truth (in the skill's `references/`) |
|---|---|
| Module boundaries, ports, contracts, composition root | `backend-modular-monolith.md` |
| Go kernel (`d.Auth`, `d.Authz`, `d.Config`, versions), module contract, registry | `backend-go.md` |
| Frontend structure, `FrontendModule`, shadcn/ui, tokens, UI kits | `frontend.md` |
| How projects use this library | `module-library.md` |

If a module needs something the architecture does not allow, the answer is a change to the
skill first (with a kernel version bump when the module contract changes), never a module that
works around it.

## Rules

`node scripts/catalog.mjs --check` enforces every rule marked ✓ and names the rule in its
message. The pre-commit hook and CI run it with `--since`, which adds the history rules.

| Rule | What it means | Checked |
|---|---|---|
| `no-module-imports` | A module never imports another module — not its internals, not its `contracts/`, in Go or in the web (`@/modules/<other>`, or a relative path out of its folder). What it needs is a port in its own `ports/`; what it offers is its `contracts/`. | ✓ |
| `adapters-declared` | An adapter (`adapters/*.go`, package `modules`) joins this module's `ports` to one provider's `contracts` and is declared under that port in `module.json`. Mapping only, no logic. | ✓ |
| `kernel-only` | Go code builds on the kernel (`internal/kernel`), `internal/config`, and `internal/shared/*` — nothing else of the project. | ✓ |
| `go-module-contract` | `api/<id>.module.go` holds `New(d, ports...) (*Module, error)`, `Routes(mux)`, and `Client()` when it offers a contract. | ✓ (file) · testbed (shape) |
| `go-layers` | `api/` holds only `contracts/ ports/ application/ domain/ infrastructure/ presentation/`. Domain, contracts, and ports import nothing of the module; application never imports infrastructure or presentation; presentation reaches storage only through application; only infrastructure touches `database/sql`. | ✓ |
| `sqlc-not-orm` | Data access is `database/sql` + sqlc (`infrastructure/queries`). No ORM. | ✓ |
| `declared-go-deps` | Every third-party Go import is in `go_dependencies` (module path → the version it was tested with), except the kernel's own (`go-sql-driver/mysql`, `joho/godotenv`). The installer pins them in the project's `go.mod`. | ✓ |
| `config-through-kernel` | Settings come from `config.Get` / `config.Require`, never `os.Getenv`. A required one fails `New`. | ✓ |
| `auth-through-kernel` | Guard handlers with `d.Auth.Require` / `d.Authz.Require("<id>.<action>")`, declare permissions with `d.Authz.Declare` in `New` and in `module.json`, hide UI with `useCan`. Never a port for auth. | names ✓ · testbed |
| `own-tables-only` | Migrations and `infrastructure/queries` create, alter, read, and write only tables in `owns_tables`, and no table is owned by two modules. No foreign key or join to another module's table — relations are primitive IDs; ask the owner through a port. Data other modules point at is deactivated, never deleted. | ✓ (tables) · review (deactivate) |
| `migrations` | `NNN_snake_case.up.sql` + `.down.sql`, numbered 001, 002, … without gaps. Only fullstack and backend modules have them. | ✓ |
| `module-layout` | A module holds only `module.json`, `README.md`, and `api/ web/ shared/ migrations/ adapters/`, as its kind allows (fullstack: api+web; backend: api; web: web, optionally shared; ui: shared, optionally a web provider). | ✓ |
| `install-targets` | `api` → `apps/api/internal/modules/<id>`, `web` → `apps/web/src/modules/<id>`, `shared` → `apps/web/src/shared`. Nothing else is written into a project except between the `elco:*` markers. | ✓ |
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
| `release-tags` | Every released version has the tag `<id>/v<version>` (the post-commit hook creates it; push with `git push --follow-tags`). A project whose recorded commit is gone upgrades from the tag. | ✓ CI on push |

## Adding or changing a module

1. Start from the closest existing module (`users` for a fullstack CRUD module, `ui-data` for a
   kit) and keep its shape. Read [docs/MODULE_FORMAT.md](docs/MODULE_FORMAT.md) for `module.json`.
2. Build the UI from the kits. If a generic piece is missing, add it to the right kit (and to
   `ui-gallery`) in its own change.
3. Bump the version, add a `## Changes` line to the README.
4. `node scripts/catalog.mjs` (regenerates `CATALOG.md`), then `npm run check`.
5. `npm run testbed` (needs MySQL; see README › Working on the library). A new module is not
   done until a testbed scenario installs it.
6. Commit. The pre-commit hook runs the check again; the post-commit hook tags the new versions.
   Push with `git push --follow-tags`. CI runs the check, the tags check, and the testbed.

Never bypass the hook (`git commit --no-verify`) to land a rule violation. If a rule is wrong,
change the rule in `scripts/lib/rules.mjs` and this table in the same commit, and say why.

## One-time setup in a clone

```bash
git config core.hooksPath .githooks   # pre-commit: the rules; post-commit: release tags
```
