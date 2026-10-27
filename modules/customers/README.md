# customers

A company's customers. Fullstack (Go + React), `scoped`, owns `cust_customers`, `cust_addresses`,
`cust_contacts`, `cust_settings`. Go kits: `money`, `taxid`. Plan:
[PLAN-MODUL-PELANGGAN-PEMASOK](../../docs/plans/PLAN-MODUL-PELANGGAN-PEMASOK.md).

**What it keeps.**
- Identity: code, name, legal name (what a tax invoice prints), party type (company,
  individual, government), foreign or not with a country.
- Tax identity as Coretax names it: `npwp` (16 digits; a 15-digit old NPWP is stored with a
  leading `0`), `nik` (resident individuals), `passport` (foreign individuals), or `none`.
  Also PKP and VAT collector (none, government, other — transaction codes 02/03).
- Billing and shipping addresses, each with an optional NITKU.
- Contacts, payment days (empty = cash), a credit limit (empty = no limit), and on hold with a
  reason.

**What it does not.** Receivables, credit-limit enforcement, tax invoices, and price lists
belong to the modules that read customers through their ports (sales, accounting, tax, items).
Tax certificates and withholding are the tax module's. Each company keeps its own customers.

**Rules.**
- Codes are typed or generated per company (`PLG-00001`; the prefix is set on the list page).
  They are not `numbering`'s: master data needs no gapless series.
- A tax ID another customer of the company already has is a 409 listing them; saving again
  with `confirm_duplicate` keeps it — one NPWP may have several places of business billed
  apart.
- Customers and addresses are **deactivated, never deleted**: documents point at them.
  Contacts are deleted, since nothing points at one.
- **Personal data (UU PDP).** NIK and passport numbers come masked in every list and detail.
  `GET /{id}/reveal` returns the whole one to holders of `customers.sensitive` across the
  company, and writes an `access` audit record. A company's NPWP shows whole.
- Setting payment terms or a credit limit needs `customers.credit` across the company, and a
  change is a `security` audit record.

## How another module uses it

Declare a port in your module with the calls you need and map it to `contracts.CustomerClient`
in an adapter in the composition root:

| Call | When |
|---|---|
| `Usable(ctx, id)` | Inside your posting's `d.Tx`, before posting. `ErrNotFound`, `ErrInactive`, `ErrOnHold`. It holds the customer's row shared until the transaction ends, so a deactivation or hold waits for your posting |
| `Snapshot(ctx, id, addressID)` | When posting: copy the name, tax ID (whole), PKP, VAT collector, and the address (0 = default billing) into your document |
| `Get(ctx, id)` | Show a customer, active or not; payment days and credit limit for your own checks |
| `Search(ctx, q, limit)` | A picker; active customers only |

Store `customer_id` as a plain ID (no foreign key). A web page of yours may call
`GET /api/v1/customers/lookup?q=` for a picker.

## Permissions

| Permission | For |
|---|---|
| `customers.read` | See customers (NIK and passport masked) |
| `customers.manage` | Add, change, hold, deactivate; addresses and contacts; the code prefix (across the company) |
| `customers.credit` | Payment terms and credit limit — across the company |
| `customers.sensitive` | See a whole NIK or passport number — across the company; each view is audited |

## Pages

- **Pelanggan** (`/customers`, nav "Penjualan", order 190): search by code, name, or NPWP;
  filter Aktif / Ditahan / Nonaktif / Semua; add a customer; the code prefix.
- **Detail** (`/customers/:id`): identity and tax, addresses, contacts, terms; change, hold or
  release, deactivate or activate.

## Changes

- **0.1.0** — first version: customers with tax identity, addresses with NITKU, contacts,
  terms and credit limit, hold, generated codes, masked personal data with audited reveal,
  `CustomerClient`.
