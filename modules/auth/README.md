# auth

Email + password sign-in with server-side sessions. Fullstack (Go + React), owns
`auth_sessions`.

**How a session works.** `POST /api/v1/auth/login` checks the password (bcrypt), stores a
session row, and sets an HS256 JWT naming that row (`jti`), the user (`sub`), and this app
(`aud` = `APP_NAME`) in an `HttpOnly`, `SameSite=Lax` cookie — `__Host-session` in production
(`Secure`, host-only, so a sibling subdomain cannot plant or overwrite it), `session` on plain-HTTP
development. A non-browser client sends `"return_token": true` to get the token in the body and
uses `Authorization: Bearer`. Every guarded request checks the token, the session row, and the
user: a session ends on **sign-out** (`POST /api/v1/auth/logout` revokes the row, so a copied
token stops working too), on **deactivation** or deletion, and when the user's **password
changes** (sessions opened before `PasswordChangedAt` are refused). `GET /api/v1/auth/me` returns
the signed-in user. Expired rows are cleared a batch at a time on each sign-in.

**Hardening.** One message for every failed sign-in; an unknown email costs as much time as a
wrong password. Each attempt is counted before the password is checked (no parallel slip-through)
against three limits per 15 minutes: 5 per email from one address, 20 per address over any
accounts, 30 per account from anywhere — the address is the real client behind nginx
(`TRUSTED_PROXIES`). Emails over 254 and passwords over 72 bytes are refused before any work, and
the limiter's memory is capped. `JWT_SECRET` must be ≥ 32 characters in production or the API
does not start. Cross-site writes and non-JSON bodies are refused by the kernel (v4).

**Kernel.** It fills `d.Auth` (`d.Auth.Provide(authModule.Authenticator())`): every module
guards its routes through that slot, never by importing auth.

**Port** `ports.UserStore` — `FindCredentials(email)`, `FindUser(id)` (its `PasswordChangedAt` may stay zero for a store that does not track it). Ready-made adapter for the
`users` module: `adapters/userstore_users.go`. Any other source works with an adapter of your own.

**Web:** `/login` (split layout: brand panel in `--sidebar`, form on the right), the registry
`guard` (`RequireAuth`: loads the session, redirects to `/login?next=…`, signs out on any 401),
and `UserMenu` in the `"sidebar-footer"` slot. Replaces the scaffold's starter `auth` module.

**Not in this version:** refresh tokens, password reset (needs a mail module), 2FA, SSO.

## Changes

- **0.4.0** — needs kernel 4. Server-side sessions (`auth_sessions`, migration 001): sign-out
  revokes the token, a password change ends earlier sessions; `__Host-session` cookie in
  production; JWT `jti` + `aud`; the token is returned only when asked (`return_token`); layered,
  race-free, memory-capped sign-in limits including per address. **Breaking:** tokens issued by
  0.3 are no longer accepted — everyone signs in once after the upgrade. `ports.User` gains
  `PasswordChangedAt` (an existing project adapter still compiles; leave it zero if unknown).

- **0.3.0** — needs kernel 3. Sign-in page on `ui-forms` (FormField, PasswordInput) and
  `ui-core` (Notice); Indonesian copy and API messages.

- **0.2.0** — needs kernel 2. Emits the app event `session:changed` on sign-in, sign-out, and
  expiry, so other modules (rbac's permissions) reload without importing auth.
