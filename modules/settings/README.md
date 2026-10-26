# settings

Settings per company (kernel 6 scope). Fullstack (Go + React), owns `settings` (named JSON documents).

**Company profile** — name, legal name, address, phone, email, NPWP (15 or 16 digits), time zone
(default `Asia/Jakarta`; it decides where a day starts for reports), and a logo. Read by any
signed-in user (`GET /api/v1/settings/company`), changed with `settings.manage`. Saving reads and
writes under a row lock, so two people saving at once never lose each other's change.

**Offers** `contracts.SettingsClient` — `Company(ctx)` and `Logo(ctx)` — for modules that print
letterheads, invoices, or emails, through their own port.

**Logo** — optional port `FileStore` (ready-made adapter: `files`). PNG, JPEG, or WebP up to 2 MB,
uploaded with `PUT /api/v1/settings/company/logo` (multipart, streamed to the file store);
replacing it deletes the old file. `GET /api/v1/settings/company/logo` is public — sign-in pages
and emails show it — with `nosniff` and a sandboxing CSP. Without the port the logo section is
hidden and everything else works.

**Settings page** `/settings` ("Pengaturan › Umum", `settings.manage`): the company form, the logo,
then whatever other modules contribute through the **`settings:sections`** slot (a
`<SectionCard>` each) — one place for settings without modules importing each other.

**Audit:** `settings.company_updated` (only the fields that changed), `settings.logo_updated`,
`settings.logo_removed`.

**Not in this version:** several companies (tenants), per-user preferences.

## Changes

- **0.2.0** — needs kernel 6. Scoped: each company keeps its own settings and company profile
  (migration 002: `company_id` in the key; existing settings are company 1's). Saving runs
  inside `d.Tx`. The public logo is company 1's (the sign-in page has no company yet).
- **0.1.0** — first version.
