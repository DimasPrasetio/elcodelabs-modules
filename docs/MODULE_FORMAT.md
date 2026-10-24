# Module format

Every module is a folder under `modules/<id>/` with a `module.json`, a `README.md`, and the
files it installs. A theme preset is a folder under `themes/<name>/` with a `preset.json`.
`scripts/catalog.mjs` validates every manifest and generates `CATALOG.md` and `catalog.json`.

The format is JSON rather than YAML so the tooling stays dependency-free, like the
`monorepo-elcodelabs` scaffolder it installs into.

## Rules every module follows

1. **A module never imports another module** — not its internals, not its `contracts/`. What it
   needs from outside is a port in its own `ports/` (Go) or goes through `shared/` (web). Ports
   are connected in the project's composition root by adapters
   (`apps/api/internal/modules/adapter_<consumer>_<port>.go`).
2. **Authentication is the kernel's `d.Auth`**, not a port: a module guards handlers with
   `d.Auth.Require` and reads the user with `d.Auth.UserID`. The auth module fills the slot.
3. **It builds only on the kernel** (`requires_kernel`) and on `ui` kits listed in `ui`.
4. **Frontend contributes through `FrontendModule`** (`module.tsx`): routes, `nav`, `slots`,
   and at most a `guard` or a `layout`.
5. **Semantic token classes only**, no raw colors — it must look right under any preset.
6. **It owns its tables**; migrations create only those. No foreign key or join to a table it
   does not own.

## `module.json`

```jsonc
{
  "id": "auth",                       // folder name; lowercase, digits, dashes
  "version": "0.1.0",                 // semver; tagged in git as <id>/v<version>
  "kind": "fullstack",                // fullstack | backend | web | ui
  "title": "Authentication",
  "summary": "One line: what it does.",
  "when_to_use": ["…"],               // what makes it the right pick
  "when_not_to_use": ["…"],           // when to build something else instead
  "stacks": ["go"],                   // backend stacks it ships for (empty for web/ui)
  "requires_kernel": 1,               // kernel.Version it is built against

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
          "expr": "authUserStore{users: usersModule.Client()}"
        }
      }
    }
  ],
  "ui": ["ui-core"],                  // ui kits its frontend is built on

  "owns_tables": ["users"],
  "env": [
    { "name": "JWT_SECRET", "required": true, "description": "…" }
  ],
  "routes": {
    "api": ["POST /api/v1/auth/login"],
    "web": ["/login"]
  },
  "npm_dependencies": { "sonner": "^2.0.8" },

  "install": {                        // folder in the module → path in the project
    "api": "apps/api/internal/modules/auth",
    "web": "apps/web/src/modules/auth"
  },
  "register": {
    "go": {
      "import": "internal/modules/auth",
      "build": ["authModule := auth.New(d, {{port:UserStore}})"],
      "routes": ["authModule.Routes(mux)"],
      "after": ["d.Auth.Provide(authModule.Authenticator())"]
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
| `ui` | `web/` into `apps/web/src` (shared kit: primitives, components, layouts) | nothing |

### Placeholders

- `__GO_MODULE__` in any `.go` file becomes the project's Go module path.
- `{{port:<Name>}}` in `register.go.build` becomes the adapter expression chosen for that port.

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
