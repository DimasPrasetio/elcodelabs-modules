# auth

Email + password sign-in. Fullstack (Go + React), owns no table.

**How a session works.** `POST /api/v1/auth/login` checks the password (bcrypt) and sets an
HS256 JWT in an `HttpOnly`, `SameSite=Lax` cookie named `session` (`Secure` in production). The
same token is in the response for non-browser clients, which send `Authorization: Bearer`.
Every guarded request re-checks that the user still exists and is active, so deactivation takes
effect on the next request. `POST /api/v1/auth/logout` clears the cookie;
`GET /api/v1/auth/me` returns the signed-in user.

**Hardening.** One message for every failed sign-in; an unknown email costs as much time as a
wrong password; after 5 failures in 15 minutes an email is refused (429) until the window
passes; `JWT_SECRET` must be ≥ 32 characters in production or the API does not start.

**Kernel.** It fills `d.Auth` (`d.Auth.Provide(authModule.Authenticator())`): every module
guards its routes through that slot, never by importing auth.

**Port** `ports.UserStore` — `FindCredentials(email)`, `FindUser(id)`. Ready-made adapter for the
`users` module: `adapters/userstore_users.go`. Any other source works with an adapter of your own.

**Web:** `/login` (split layout: brand panel in `--sidebar`, form on the right), the registry
`guard` (`RequireAuth`: loads the session, redirects to `/login?next=…`, signs out on any 401),
and `UserMenu` in the `"sidebar-footer"` slot. Replaces the scaffold's starter `auth` module.

**Not in this version:** refresh tokens, password reset (needs a mail module), 2FA, SSO.

## Changes

- **0.2.0** — needs kernel 2. Emits the app event `session:changed` on sign-in, sign-out, and
  expiry, so other modules (rbac's permissions) reload without importing auth.
