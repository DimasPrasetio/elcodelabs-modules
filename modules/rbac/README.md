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
- **Per company and branch** (kernel 6 scope) — roles are defined once; a user holds a role in
  one company, across all of it (`branch_id` 0) or in one branch. Where a request works
  (`kernel.ScopeFrom`) decides what applies: in a branch, the company-wide roles and that
  branch's; at the whole company, company-wide roles only — someone holding a role in one
  branch works in that branch's scope. `d.Authz.Units` (rbac is a `kernel.ScopedAuthorizer`)
  tells a module which branches a permission reaches. Managing counts only what the manager
  holds *there*: giving or taking a role in a branch needs `rbac.manage` and the role's
  permissions in that branch, and the user placed there (`d.Scope`; without an organization
  module only company 1 exists, with no branches); defining, changing, or deleting a role —
  defined once for the app — needs them company-wide in every company where the role has
  members, under a lock on the role, so nobody gives it elsewhere in between. An account belongs
  wherever it holds a role and — with the `Placements` port — wherever it works; with neither,
  to company 1 as a whole. Acting on it (`Covers`: reset a password, deactivate) needs, in every
  one of those places, a standing of one's own and everything the account holds there — across
  a company, company-wide; in a branch, in that branch. One company never reaches an account
  another company shares; a branch administrator reaches only accounts that live in their
  branch. An Administrator across company 1 — the home company, where an `organization` module
  manages companies — administers the whole installation: they hold every permission in every
  company (still entering only where they are placed), so no company, not even one a colleague
  created, is beyond them. Each company
  keeps at least one active company-wide Administrator (checked under a lock, so two removals at
  once cannot both pass); the first one is bootstrapped in company 1 — another company's first
  is its creator, appointed when the `organization` module announces it.

**API:** `GET /api/v1/rbac/me` (the signed-in user's permissions — session only) and, behind
`rbac.manage`: permission catalog, roles CRUD, role members (list/add/remove), user search.

**Port** `ports.UserDirectory` — `Get`, `FindByEmail`, `Search`. Ready-made adapter for the
`users` module (needs users ≥ 0.2.0): `adapters/userdirectory_users.go`.

**Web:** `/roles` (list: permissions count, members) and `/roles/:id` (permission checkboxes
grouped by module, members with add/remove), nav "Roles" in "Settings" shown with
`rbac.manage`. Its `provider` supplies `@/shared/access` — `useCan` and `NavItem.permission` in
every module answer from the user's real permissions, reloaded on the `session:changed` and
`scope:changed` app events. A member held in one branch shows "hanya cabang #n"; giving a role
in one branch is the API's `branch_id` (an organization module brings the branch picker).

## Changes

- **0.6.0** — With an `organization` module: the creator of a new company becomes its
  Administrator (listens to `organization.company.created`) — how a second company gets its
  first one; optional late port `Placements` — `Covers` counts where an account works as well as
  where it holds roles (branch-aware: a branch administrator reaches accounts that live in
  the branch), and the
  member search lists the active company's people. Wire it with
  `module.mjs upgrade rbac --port Placements=organization`.
- **0.5.0** — needs kernel 6. Roles held per company and branch (migration 003: `company_id`,
  `branch_id` on `user_roles`; existing members stay company-wide in company 1), `Units` for
  `d.Authz.Units`, membership changes recorded as `security` audit, repositories inside `d.Tx`.
  At the whole company only company-wide roles apply; managing a role needs `rbac.manage`
  where it applies (in every company a role's change reaches); `Covers` looks across
  companies; the last active company-wide Administrator is kept under a lock.
- **0.4.0** — needs kernel 5. Role changes and membership are recorded in `d.Audit`
  (`role.created`, `role.updated` with the before/after permissions, `role.deleted`,
  `role.member_added`, `role.member_removed`).

- **0.3.0** — needs kernel 4. `rbac.manage` is no path to more permissions: creating, editing,
  or deleting a role, and giving or taking one away, need every permission the role grants (the
  Administrator role: being an administrator). Implements `Authorizer.Covers`, so modules such as
  `users` keep operators off administrators' accounts. The bootstrap gives the Administrator role
  again when no *active* user holds it.

- **0.2.0** — needs kernel 3. Built on the four UI kits (data table, section cards, modal,
  user picker with server search, confirm dialogs, toasts); Indonesian copy, API messages, and
  permission descriptions. Migration 002 translates the Administrator role's description.

**Not here:** per-record access (e.g. only your branch's data) — that belongs in the module that
owns the data; permission groups/inheritance.
