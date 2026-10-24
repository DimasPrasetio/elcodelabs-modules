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
architecture.

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
  "requires_kernel": 3,               // minimum kernel.Version it needs

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
  "permissions": [                    // what it declares with d.Authz.Declare — <id>.<action>
    { "name": "users.manage", "description": "Add, edit, deactivate, and delete users" }
  ],

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
