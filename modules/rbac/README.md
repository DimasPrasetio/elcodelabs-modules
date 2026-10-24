# rbac

Role-based access control. Fullstack (Go + React), owns `roles`, `role_permissions`,
`user_roles`.

**How it fits.** Modules *declare* the permissions they guard with
(`d.Authz.Declare`, named `<module>.<action>`) and guard routes with `d.Authz.Require`. Without
rbac the kernel's policy is "signed in = allowed". Installing rbac fills `d.Authz`
(`d.Authz.Provide(rbacModule.Authorizer())`): from then on a user may do what their roles grant.
Permissions come from code; roles and who holds them are data admins edit.

- **Administrator** — created by the migration, holds `*` (every permission, including ones
  modules add later), cannot be renamed, changed, or deleted, and always keeps at least one
  member.
- **First administrator** — while nobody holds Administrator, startup gives it to
  `RBAC_BOOTSTRAP_ADMIN_EMAIL` (default `USERS_BOOTSTRAP_EMAIL`), so installing rbac never locks
  everyone out.
- **Effective immediately** — a user's permissions are cached for 30 seconds and the cache is
  cleared on every role or membership change.
- `user_roles.user_id` is a primitive ID (no foreign key across modules); a member the user
  directory no longer knows is listed as missing, so it can still be removed.

**API:** `GET /api/v1/rbac/me` (the signed-in user's permissions — session only) and, behind
`rbac.manage`: permission catalog, roles CRUD, role members (list/add/remove), user search.

**Port** `ports.UserDirectory` — `Get`, `FindByEmail`, `Search`. Ready-made adapter for the
`users` module (needs users ≥ 0.2.0): `adapters/userdirectory_users.go`.

**Web:** `/roles` (list: permissions count, members) and `/roles/:id` (permission checkboxes
grouped by module, members with add/remove), nav "Roles" in "Settings" shown with
`rbac.manage`. Its `provider` supplies `@/shared/access` — `useCan` and `NavItem.permission` in
every module answer from the user's real permissions, reloaded on the `session:changed` app event.

**Not here:** per-record access (e.g. only your branch's data) — that belongs in the module that
owns the data; permission groups/inheritance.
