# users

The people who can sign in. Fullstack (Go + React), owns the `users` table.

**API** (every route behind `d.Auth.Require`):
`GET /api/v1/users?page=&limit=&q=` (paginated, searches name/email) · `POST /api/v1/users` ·
`GET|PUT|DELETE /api/v1/users/{id}`. Emails are stored lower-cased and unique; passwords are
bcrypt hashes (8–72 characters). Nobody can deactivate or delete their own account (409).

**First user.** With `USERS_BOOTSTRAP_EMAIL` and `USERS_BOOTSTRAP_PASSWORD` set, the module
creates that user at startup when the table is empty. Remove both after the first sign-in.

**Offers** `contracts.UserClient` — `GetByID`, `GetCredentialsByEmail` (for an auth provider).
It imports no other module.

**Web:** `/users` — list with search, pagination, status pill; add/edit in a dialog, delete
with confirmation; nav entry "Users" in the "Settings" group. Built on `ui-core`.

**Not here:** roles and permissions (a separate module), self-service profile/password change.
