# Rule: Modules and registries

Applies to: `apps/api/internal/**`, `apps/api/sqlc.yaml`, `apps/web/src/modules/**`, `apps/web/src/app/**`.

- A module is wired in only through its registry: `apps/api/internal/modules/registry.go`
  (backend) and `apps/web/src/app/modules.ts` (frontend). Never mount it anywhere else.
- Keep every `elco:*` marker. Tooling inserts lines between them — add yours between them too,
  never around or instead of them.
- A frontend module never imports another module. Shared needs go through `shared/`; putting
  several modules on one screen happens in `app/`. Hide what needs a permission with `useCan`
  or `NavItem.permission`; tell other modules something happened with `@/shared/app-events`.
- Go: a module's package root exposes `New(d *kernel.Deps, <one value per port it needs>...) (*Module, error)`,
  `Routes(mux *http.ServeMux)`, and `Client()` when it offers a contract. `internal/modules` is
  the composition root: `registry.go` builds a provider before the modules wired to it, and
  each connection is an adapter in its own `adapter_<consumer>_<port>.go` beside it.
- Go: guard a handler with `d.Auth.Require(h)` and read the user with `d.Auth.UserID(ctx)`. The
  auth module fills that slot in `registry.go` with `d.Auth.Provide(...)` — never import it.
- Go: a module declares its permissions (`<module>.<action>`) with `d.Authz.Declare` in `New` and
  guards with `d.Authz.Require(permission, h)`; never check roles or import an rbac module.
- Go: a module owns its `sql:` block in `sqlc.yaml` and adds migrations with
  `npm run migrate:create` (timestamp-versioned, so two branches never collide).
- Go: `internal/kernel` and `internal/shared` are the kernel every module builds on. Change them
  only while every module still builds, and bump `kernel.Version` for a change that would
  break an existing module.
- Customize an installed module in new files of its own (e.g. `custom_*.go`, `Custom*.tsx`)
  rather than editing the files it shipped with, so a later upgrade merges cleanly.
- Library modules are installed and upgraded only with the library's `module.mjs` (`add`,
  `upgrade`, `detach`) — never copied by hand. `modules.lock.json` records them; commit it.
