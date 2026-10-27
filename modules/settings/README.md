# settings

Settings per company (kernel 6 scope). Fullstack (Go + React), owns `settings` (named JSON documents).

**Company profile** — name, legal name, address, phone, email, NPWP (15 or 16 digits), and a
logo, for the company the request works in. Read by any
signed-in user (`GET /api/v1/settings/company`), changed with `settings.manage`. Saving reads and
writes under a row lock, so two people saving at once never lose each other's change.

**Offers** `contracts.SettingsClient` — `Company(ctx)` and `Logo(ctx)` — for modules that print
letterheads, invoices, or emails, through their own port.

**Logo** — optional port `FileStore` (ready-made adapter: `files`). PNG, JPEG, or WebP up to 2 MB,
uploaded with `PUT /api/v1/settings/company/logo` (multipart, streamed to the file store);
replacing it deletes the old file. `GET /api/v1/settings/company/logo?company=<id>` (default 1)
is public — sign-in pages and emails show it — with `nosniff` and a sandboxing CSP. Without the port the logo section is
hidden and everything else works.

**Settings page** `/settings` ("Pengaturan › Umum", `settings.manage`): the company form, the logo,
then whatever other modules contribute through the **`settings:sections`** slot (a
`<SectionCard>` each) — one place for settings without modules importing each other.

**Audit:** `settings.company_updated` (only the fields that changed), `settings.logo_updated`,
`settings.logo_removed`.

**Time zone** — no longer set here: where a day starts follows `kernel.Today`, the scope's zone
(a branch's or company's, from the `organization` module), else `APP_TIMEZONE`. The contract's
`Timezone` keeps the stored value and is deprecated.

**Not in this version:** per-user preferences.

## Changes

- **0.3.0** — Ready for several companies (`organization`): the public logo names its company
  (`?company=<id>`, default 1) instead of reading a scope a public request does not have; the
  profile says which company it is (`company_id`), so the settings page shows that company's
  logo. The time zone leaves the form (deprecated; see above).
- **0.2.0** — needs kernel 6. Scoped: each company keeps its own settings and company profile
  (migration 002: `company_id` in the key; existing settings are company 1's). Saving runs
  inside `d.Tx`. The public logo is company 1's (the sign-in page has no company yet).
- **0.1.0** — first version.
