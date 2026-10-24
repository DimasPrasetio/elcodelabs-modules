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

**Web:** `/users` — the Bingkai Navy list template: toolbar search with a filter chip, data
table (status pill, ⋯ row menu), pagination with rows per page; add/edit in a side sheet,
delete with confirmation, toasts on success; nav entry "Pengguna" in "Pengaturan" (shown with
`users.read`); changing shown with `users.manage`. Indonesian copy. Built on `ui-core`,
`ui-data`, `ui-forms`, `ui-feedback`.

**Not here:** roles and who holds which permission (the `rbac` module), self-service
profile/password change.

## Changes

- **0.3.0** — needs kernel 3. Built on the four UI kits (list template, form sheet, toasts);
  Indonesian copy and API messages.
- **0.2.0** — needs kernel 2. Routes guarded by `users.read` / `users.manage` instead of "signed
  in"; UI hides what the user may not do. `UserClient` gains `GetByEmail` and `Search`.
