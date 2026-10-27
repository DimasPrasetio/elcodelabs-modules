# organization

Several companies, their branches, and where each person works. Fullstack (Go + React), owns
`org_companies`, `org_branches`, `org_placements`.

**What it does.** A **company** is a legal entity; a **branch** is one of its places of business
(not a legal entity: no NPWP of its own — it has a NITKU, the head office's 16-digit NPWP plus a
6-digit number). A user is **placed** across a company or in one of its branches, possibly in
several places. The module fills the kernel's **`d.Scope`**: every signed-in request works in one
company and, optionally, one branch (`kernel.ScopeFrom(ctx)`), checked here against the user's
placements — the web asks for one with `X-Scope` and is refused (403) anywhere else. The
scope's time zone is the branch's, else the company's: `kernel.Today` gives a Makassar branch its
own date. Background work runs per company (`d.Scope.Companies`, active companies only).

| Asked (`X-Scope`) | Answer |
|---|---|
| nothing | The user's default placement (marked in the picker; else the first across a company, then the first branch) |
| `C` | Company C as a whole — only for someone placed across it |
| `C:B` | Branch B of C — for someone placed across C, or in B |
| a deactivated company or branch, anything else | 403 |

A placement opens the **door** to a place; what someone may do there is still their roles
(`rbac`, which gives roles per company and branch).

**Companies** are created, renamed, and deactivated from the **home company** (company 1) as a
whole, by someone holding `organization.companies` there. From there, someone may also be placed
by exact email across another company (Perusahaan › "Tempatkan pengguna…") — with what placing
always needs: `organization.manage` in that company and authority over the account, so in
practice by the home company's Administrators or the company's founder. It is how a company whose
administrators can no longer get in gets one back. With `rbac`, an Administrator across the home
company administers the whole installation. The creator is placed across the new
company, and — through the `organization.company.created` event — `rbac` makes them its
Administrator. The home company cannot be deactivated; a deactivated company's data stays, nobody
works in it, and its background work stops.

**Branches and placements** of the active company need `organization.manage`: across the whole
company for branches and company-wide placements; reaching the branch (`d.Authz.Units`) for a
branch placement. Placing or unplacing someone also needs authority over their account: their
placements — in active units or not, so deactivating a branch or company hands its people to no
one; none — the home company — are where it belongs, and the actor
needs `organization.manage` in every one of them plus `d.Authz.Covers` (with `rbac`: everything
they hold, where they hold it). A placement is thus no way to claim another company's account,
shield it from its own administrators, or lock administrators out; and to anyone without the
home company's authority, an account that does not exist answers the same 403 as one that is
not theirs. A branch head manages placements of people who live in their branch. Nobody
changes their own placements. Placing someone from another
company finds them by their **exact email** — a partial search lists only people already placed
here. The Penempatan page shows the whole company to someone reading it company-wide, and only
their branches' placements otherwise.

**Installing it** turns the app into one with several companies:

1. The migration creates company 1 — the one all existing data belongs to (`company_id DEFAULT
   1`) — named after `APP_NAME`, in `APP_TIMEZONE`, on the first start.
2. On that start every existing user is placed across company 1: nothing changes for anyone.
3. **Wire the late ports** of the modules that use placements (installing organization lists
   them in its notes), so new users are placed and lists follow the company — until then, users
   pages list every company's people:
   ```bash
   node scripts/module.mjs upgrade users --port Placements=organization
   node scripts/module.mjs upgrade rbac --port Placements=organization
   ```
   Without `users`' port, a user created later is placed nowhere (they appear under "Tidak bisa
   bekerja di mana pun" on the Penempatan page, for the home company's administrators — with
   anyone placed only in deactivated companies or branches).
4. `settings` must be 0.3.0 or newer: older versions read a scope on the public logo route,
   which has none once there are several companies.

To remove the module later, unwire them first (`--port Placements=none`), then
`module.mjs remove organization`.

Isolation between companies rests on an authorization module (`rbac`): without one, the
kernel's default is "signed in = allowed", in every company the user is placed in.

From then on there is no default company: `kernel.ScopeFrom` outside a request — a job, a public
route — panics instead of filing data under company 1. Code that works per company outside a
request loops `d.Scope.Companies` with `kernel.WithScope`.

**Offers** `contracts.OrganizationClient` — `Company`, `Branch`, `Branches`, `PlacedUserIDs`,
`IsPlaced`, `PlacesOf`, `PlaceNew` — for modules that print a branch's name, number documents
per branch, or list the active company's people (`users`, `rbac`, through their own ports).

**Needs** port `UserDirectory` (required; adapter `users` ≥ 0.7.0): users by ID or email, search
(among all, and among given IDs), and every ID.

**Emits** `organization.company.created` v1 — `{company_id, code, name, created_by}`; the creator
is already placed across the company.

**Web:** a picker in the header (slot `shell:scope`) — "PT Maju › Bandung" — switches the tab
(other tabs keep theirs) and can make a place the default; a stale stored scope (a deactivated
company, a placement taken away) is dropped before the app renders. Pages under
"Pengaturan › Organisasi": Cabang, Penempatan, and — for `organization.companies` — Perusahaan.

**Audit:** companies and branches created and changed (`change`); placements, deactivations, and
activations (`security`).

**Not in this version:** departments and reporting lines (an HR module), parent/subsidiary
companies and consolidation (an accounting module), a company chosen by subdomain for public
pages.

## Changes

- **0.1.0** — first version.
