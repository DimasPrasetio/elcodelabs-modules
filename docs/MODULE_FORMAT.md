# Module format

Every module is a folder under `modules/<id>/` with a `module.json`, a `README.md`, and the
files it installs. A theme preset is a folder under `themes/<name>/` with a `preset.json`.
`scripts/catalog.mjs` validates every manifest and generates `CATALOG.md` and `catalog.json`.

The format is JSON rather than YAML so the tooling stays dependency-free, like the
`monorepo-elcodelabs` scaffolder it installs into.

## Rules every module follows

The architecture rules — what a module may import, how its layers depend on each other, which
tables it may touch, how its UI is built, and what a change may not do to a released module —
are listed in [AGENTS.md](../AGENTS.md), each with the check that enforces it
(`scripts/lib/rules.mjs`). They come from the `monorepo-elcodelabs` skill, which owns the
architecture; the kernel contract a module builds on is pinned, read-only, in
[`kernel/`](../kernel/README.md) (`kernel/reference/` and the host project's kernel code).

## `module.json`

```jsonc
{
  "id": "auth",                       // folder name; lowercase, digits, dashes
  "version": "0.1.0",                 // semver; tagged in git as <id>/v<version>
  "kind": "fullstack",                // fullstack | backend | web | ui | gokit
  "title": "Authentication",
  "summary": "One line: what it does.",
  "when_to_use": ["…"],               // what makes it the right pick
  "when_not_to_use": ["…"],           // when to build something else instead
  "stacks": ["go"],                   // backend stacks it ships for (empty for web/ui)
  "requires_kernel": 3,               // minimum kernel.Version it needs — at most the pinned one

  "provides": {
    "contracts": ["UserClient"],      // interfaces in api/contracts other modules can be wired to
    "kernel": ["auth"]                // kernel slots it fills ("auth" → d.Auth.Provide)
  },
  "ports": [                          // what it needs from outside (api/ports)
    {
      "name": "UserStore",
      "required": true,
      "description": "Find a user's credentials by email and their profile by ID.",
      "adapters": {                   // ready-made adapters, keyed by provider module id
        "users": {
          "file": "adapters/userstore_users.go",
          "expr": "authUserStore{users: usersModule.Client()}",
          "requires": "0.4.0"         // optional: the oldest provider version the adapter compiles
        }                             // against — install/upgrade refuse to wire an older one
      }
    }
  ],
  "ui": ["ui-core"],                  // ui kits its frontend is built on
  "go_kits": ["money"],               // Go kits its backend imports (internal/shared/<kit>)
  "permissions": [                    // what it declares with d.Authz.Declare — <id>.<action>
    { "name": "users.manage", "description": "Add, edit, deactivate, and delete users" }
  ],

  "owns_tables": ["users"],
  "scoped": false,                    // true: its data belongs to a company (kernel 6) — every
                                      // table has company_id, every query names it
  "emits": [                          // events it publishes with d.Events.Publish (kernel 6)
    { "name": "users.user.deactivated", "version": 1, "description": "{id} — fields only added within a version" }
  ],
  "listens": [                        // events it reacts to; the installer wires them in registry.go
    { "event": "hr.employee.left", "subscriber": "users.deactivate-leaver", "handler": "usersModule.OnEmployeeLeft" }
  ],
  "env": [                            // listed in .env.example and in the production example —
                                      // required ones as KEY=<required>, which deploy.sh enforces
    { "name": "JWT_SECRET", "required": true, "description": "…" }
  ],
  "routes": {
    "api": ["POST /api/v1/auth/login"],
    "web": ["/login"]
  },
  "npm_dependencies": { "sonner": "^2.0.8" },
  "go_dependencies": { "github.com/golang-jwt/jwt/v5": "v5.3.1" },
                                      // every third-party Go module it imports, at the version
                                      // it was tested with — pinned in the project's go.mod

  "install": {                        // folder in the module → path in the project
    "api": "apps/api/internal/modules/auth",
    "web": "apps/web/src/modules/auth"
  },
  "register": {
    "go": {
      "import": "internal/modules/auth",
      "build": ["authModule := auth.New(d, {{port:UserStore}})"],
      "routes": ["authModule.Routes(mux)"],
      "after": ["d.Auth.Provide(authModule.Authenticator())"]   // e.g. filling a kernel slot
    },
    "web": { "import": "@/modules/auth/module", "export": "authModule" }
  }
}
```

### Kinds

| Kind | Installs | Registers |
|---|---|---|
| `fullstack` | `api/` + `web/` (+ `migrations/`) | Go registry + web registry |
| `backend` | `api/` (+ `migrations/`) | Go registry |
| `web` | `web/` | web registry |
| `ui` | its folders into `apps/web/src` (`shared/` kit: primitives, components, lib) | nothing, or only a web provider (`register.web`, e.g. `ui-feedback`'s toaster); never Go |
| `gokit` | `lib/` into `apps/api/internal/shared/<id>` (package `<id>` without dashes) — shared Go code: no tables, no routes, no wiring, imports only the standard library, its `go_dependencies`, and other Go kits | nothing |

A **Go kit** is how backend modules share code without importing each other — the backend's
counterpart of a ui kit (money, field encryption, document states). A module that imports one
lists it in `go_kits`; installing the module installs the kit first. Code that needs a table,
a route, or the kernel's slots is a module, not a kit. A kit cannot take the name of a package
the kernel's `internal/shared` already has (see `kernel/host/apps/api/internal/shared/`), nor
`kernel` or `config`.

### Kernel 6: transactions, events, scope

- A module on kernel 6 builds its queries per call — `sqlc.New(d.Tx.Conn(ctx))` — so a caller's
  `d.Tx` is joined, never opens a transaction itself (`d.Tx.Do`), never uses the kernel's `DB`, and
  runs no SQL outside sqlc except on `d.Tx.Conn(ctx)`.
- `"scoped": true` — the module's data belongs to a company: `company_id` in every table (and
  `branch_id` where it means something), in business keys, and in every query, explicitly — in
  the `WHERE` of a SELECT/UPDATE/DELETE, in the column list of an INSERT. A system query spanning
  every company (a purge) says so with a line `-- all companies` and a name starting `System`,
  so its callers see it too. With an organization module (several companies) a ctx without a
  scope has no default company: work outside a request runs per company (`d.Scope.Companies`,
  `kernel.WithScope`).
- `emits` lists the events the module publishes (`<id>.<thing>.<past-verb>`, a version, the
  payload's fields); `listens` the ones it reacts to — `handler` is a Go expression on the
  module's registry variable taking `(ctx, kernel.Event) error`. Subscribers decode the JSON
  payload into their own struct: no import of the publisher. `d.Events.Publish` names the event
  with a string literal and its declared version, so the check can match it against `emits`.

### Placeholders

- `__GO_MODULE__` in any `.go` file becomes the project's Go module path.
- `{{port:<Name>}}` in `register.go.build` becomes the adapter expression chosen for that port —
  `nil` for an optional port (`"required": false`) left unwired, which the module's `New` treats
  as "this feature is off". A port is optional whenever the module still makes sense without it;
  adding a required port to a released module breaks every project that has not wired it.

### Migrations

`migrations/` holds `NNN_<name>.up.sql` / `.down.sql`. On install each pair is renamed to a
fresh timestamp version (`<yyyymmddhhmmss>_<id>_<name>`), in `NNN` order.

### sqlc

A module with `api/infrastructure/queries/` gets its own `sql:` block in `sqlc.yaml`, writing to
`api/infrastructure/sqlc/` (generated in the project, never committed here).

## `preset.json` (themes)

```jsonc
{
  "id": "bingkai-navy",
  "version": "0.1.0",
  "title": "Bingkai Navy",
  "summary": "…",
  "when_to_use": ["…"],
  "npm_dependencies": { "@fontsource-variable/inter": "^5.3.0" }
}
```

A preset folder holds `theme.css` (the whole file — same token names as the scaffold's contract,
its own values), `fonts.ts`, and `DESIGN.md` (the design direction a project's
`knowledge/DESIGN.md` can start from).
