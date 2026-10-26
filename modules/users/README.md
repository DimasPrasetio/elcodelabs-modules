# users

The people who can sign in. Fullstack (Go + React), owns the `users` table.

**API** — reading needs `users.read`, changing needs `users.manage` (declared with
`d.Authz.Declare`; without an authorization module any signed-in user passes):
`GET /api/v1/users?page=&limit=&q=` (paginated, searches name/email) · `GET /api/v1/users/{id}` ·
`POST /api/v1/users` · `PUT /api/v1/users/{id}` (fields left out of the body keep their value).
Emails are stored lower-cased and unique; passwords are bcrypt hashes (8–72 characters).

**Protections.** People are **deactivated, never deleted** — orders, roles, and logs elsewhere
keep pointing at them, and deactivation ends their sessions at once. Nobody can deactivate their
own account (409). Changing *someone else's* account — name, email, password, active state — is
allowed only when it holds no permission the actor lacks (`d.Authz.Covers`, kernel 4): an
operator with `users.manage` cannot reset an administrator's password or lock them out. A
password change records `password_changed_at`, and the auth provider ends every session opened
before it.

**Own profile.** `GET|PUT /api/v1/account/profile` — any signed-in user reads their profile and
changes their name. Email stays with administrators: changing it oneself is a way to take an
account over. Changing a password is the auth provider's job (it checks the current password
first) and reaches users through `UserClient.SetPassword`.

**Audit.** Every change goes to `d.Audit` (kernel 5): `user.created`, `user.updated` (what changed,
never a password), `user.deactivated`, `user.activated`, `user.profile_updated`.

**First user.** With `USERS_BOOTSTRAP_EMAIL` and `USERS_BOOTSTRAP_PASSWORD` set, the module
creates that user at startup when the table is empty. Remove both after the first sign-in.

**Offers** `contracts.UserClient` — `GetByID` (with `PasswordChangedAt`), `SetPassword` (for an
auth provider; validates 8–72 bytes, records the change), `GetByEmail`, `Search` (e.g. for assigning
roles), `GetCredentialsByEmail` (for an auth provider).
It imports no other module.

**Web:** `/users` — the Bingkai Navy list template: toolbar search with a filter chip, data
table (status pill, ⋯ row menu), pagination with rows per page; add/edit in a side sheet,
deactivate/activate with confirmation, toasts on success; `/account` — "Profil saya", linked
from the user menu through the `user-menu:items` slot; nav entry "Pengguna" in "Pengaturan" (shown with
`users.read`); changing shown with `users.manage`. Indonesian copy. Built on `ui-core`,
`ui-data`, `ui-forms`, `ui-feedback`.

**Not here:** roles and who holds which permission (the `rbac` module), passwords (the auth
provider), changing one's own email.

## Changes

- **0.5.0** — needs kernel 5. Own profile (`/api/v1/account/profile`, page `/account`, menu item
  through the `user-menu:items` slot); `UserClient.SetPassword` for an auth provider's change and
  reset flows; every change recorded in `d.Audit`.

- **0.4.0** — needs kernel 4. Migration 002 adds `password_changed_at` (a password change ends
  earlier sessions with auth ≥ 0.4). Accounts holding more permissions than the actor are off
  limits (`d.Authz.Covers`). **Breaking:** `DELETE /api/v1/users/{id}` is gone — deactivate
  instead; the list's row menu offers Nonaktifkan / Aktifkan. A `PUT` without `is_active` no
  longer deactivates the user.

- **0.3.0** — needs kernel 3. Built on the four UI kits (list template, form sheet, toasts);
  Indonesian copy and API messages.
- **0.2.0** — needs kernel 2. Routes guarded by `users.read` / `users.manage` instead of "signed
  in"; UI hides what the user may not do. `UserClient` gains `GetByEmail` and `Search`.
