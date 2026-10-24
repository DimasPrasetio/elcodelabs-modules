# users

The people who can sign in. Fullstack (Go + React), owns the `users` table.

**API** — reading needs `users.read`, changing needs `users.manage` (declared with
`d.Authz.Declare`; without an authorization module any signed-in user passes):
`GET /api/v1/users?page=&limit=&q=` (paginated, searches name/email) · `GET /api/v1/users/{id}` ·
`POST /api/v1/users` · `PUT|DELETE /api/v1/users/{id}`. Emails are stored lower-cased and unique; passwords are
bcrypt hashes (8–72 characters). Nobody can deactivate or delete their own account (409).

**First user.** With `USERS_BOOTSTRAP_EMAIL` and `USERS_BOOTSTRAP_PASSWORD` set, the module
creates that user at startup when the table is empty. Remove both after the first sign-in.

**Offers** `contracts.UserClient` — `GetByID`, `GetByEmail`, `Search` (e.g. for assigning
roles), `GetCredentialsByEmail` (for an auth provider).
It imports no other module.

**Web:** `/users` — list with search, pagination, status pill; add/edit in a dialog, delete
with confirmation; nav entry "Users" in the "Settings" group (shown with `users.read`); add,
edit, and delete shown with `users.manage`. Built on `ui-core`.

**Not here:** roles and who holds which permission (the `rbac` module), self-service
profile/password change.

## Changes

- **0.2.0** — needs kernel 2. Routes guarded by `users.read` / `users.manage` instead of "signed
  in"; UI hides what the user may not do. `UserClient` gains `GetByEmail` and `Search`.
