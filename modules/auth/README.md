# auth

Email + password sign-in with server-side sessions. Fullstack (Go + React), owns
`auth_sessions` and `auth_password_resets`.

**How a session works.** `POST /api/v1/auth/login` checks the password (bcrypt), stores a
session row, and sets an HS256 JWT naming that row (`jti`), the user (`sub`), and this app
(`aud` = `APP_NAME`) in an `HttpOnly`, `SameSite=Lax` cookie — `__Host-session` in production
(`Secure`, host-only, so a sibling subdomain cannot plant or overwrite it), `session` on plain-HTTP
development. A non-browser client sends `"return_token": true` to get the token in the body and
uses `Authorization: Bearer`. Every guarded request checks the token, the session row, and the
user: a session ends on **sign-out** (`POST /api/v1/auth/logout` revokes the row, so a copied
token stops working too), on **deactivation** or deletion, and when the user's **password
changes** (sessions opened before `PasswordChangedAt` are refused). `GET /api/v1/auth/me` returns
the signed-in user.

**Hardening.** One message for every failed sign-in; an unknown email costs as much time as a
wrong password. Each attempt is counted before the password is checked (no parallel slip-through)
against three limits per 15 minutes: 5 per email from one address, 20 per address over any
accounts, 30 per account from anywhere — the address is the real client behind nginx
(`TRUSTED_PROXIES`), and an IPv6 client counts as its /64. Emails over 254 and passwords over 72
bytes are refused before any work. The limiter's memory is capped by dropping the least recently
used keys, so a flood of addresses never locks everyone out; the password flows have a limiter of
their own. A session starts when its password is checked, so a sign-in that races a password
change does not outlive it. A failed sign-in with an unknown email is recorded masked
(`b***@example.com`): what was typed may be a password. `JWT_SECRET` must be ≥ 32 characters in production or the API
does not start. Cross-site writes and non-JSON bodies are refused by the kernel (v4).

**Passwords.** Two optional ports switch the password features on:
`PasswordWriter` (ready-made adapter: `users`) and `Mailer` (ready-made adapter: `mailer`).
- **Change my password** (`POST /api/v1/auth/password`, needs `PasswordWriter`): checks the
  current password, stores the new one (the store records when — every other session ends), and
  gives this device a fresh session so the person stays signed in here. Page `/account/password`,
  "Ubah kata sandi" in the user menu.
- **Forgot password** (`POST /api/v1/auth/forgot` and `/reset`, needs both ports): the answer is
  the same whether or not the email has an account, and as fast — the lookup and the email happen
  after it; at most 3 emails per address and 10 requests per IP in 15 minutes. The link carries a
  256-bit token in its fragment (`/reset-password#token=…`, never sent to a server, so never in
  access logs) — only its hash is stored (`auth_password_resets`) — valid for 30 minutes, once,
  and only while the account keeps the address it was sent to and its password: a newer request,
  a password change, or a new email voids it. A failed save leaves the link usable. The link is
  built from `APP_URL` (required in production with both ports), never from a request header.
  Resetting ends every session of the account. Pages `/forgot-password` and
  `/reset-password`; "Lupa kata sandi?" on the sign-in page only when it is on
  (`GET /api/v1/auth/options`).

**Audit.** `auth.signed_in`, `auth.sign_in_failed`, `auth.signed_out`, `auth.password_changed`,
`auth.password_reset_requested`, `auth.password_reset` go to `d.Audit`. Expired sessions and
reset tokens are cleared by an hourly job (`d.Jobs`).

**User menu.** Renders the `user-menu:items` slot, so other modules add items (users' "Profil saya")
without importing auth.

**Kernel.** It fills `d.Auth` (`d.Auth.Provide(authModule.Authenticator())`): every module
guards its routes through that slot, never by importing auth.

**Port** `ports.UserStore` — `FindCredentials(email)`, `FindUser(id)` (its `PasswordChangedAt` may stay zero for a store that does not track it). Ready-made adapter for the
`users` module: `adapters/userstore_users.go`. Any other source works with an adapter of your own.

**Web:** `/login` (split layout: brand panel in `--sidebar`, form on the right), the registry
`guard` (`RequireAuth`: loads the session, redirects to `/login?next=…`, signs out on any 401),
and `UserMenu` in the `"sidebar-footer"` slot. Replaces the scaffold's starter `auth` module.

**Not in this version:** refresh tokens, 2FA, SSO, passwordless sign-in.

## Changes

- **0.6.0** — needs kernel 6. Sessions and reset tokens stored inside `d.Tx`; sign-ins, failed
  sign-ins, sign-outs, and password changes recorded as `security` audit (UU PDP: who got in,
  and who tried). Every guarded request now carries its company/branch scope (resolved by the
  kernel in `d.Auth.Require`).
- **0.5.0** — needs kernel 5. Change-my-password and forgot-password (optional ports
  `PasswordWriter`, `Mailer`; table `auth_password_resets`, migration 002; `APP_URL`); audit of
  sign-ins and password changes; hourly cleanup job; the user menu renders `user-menu:items`;
  sign-in, forgot, and reset pages share one layout. `auth.New` takes the two new ports — an
  upgrade rewrites the registry line (nil for a port left unwired). The `PasswordWriter` adapter
  for `users` needs users 0.5.0 (`SetPassword`): upgrade users first — the installer refuses
  otherwise. Limits per IPv6 /64, a limiter that drops old keys instead of refusing new ones, and
  failed sign-ins recorded with the email masked.

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
