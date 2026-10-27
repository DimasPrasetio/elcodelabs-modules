# suppliers

A company's suppliers. Fullstack (Go + React), `scoped`, owns `supp_suppliers`, `supp_addresses`,
`supp_bank_accounts`, `supp_contacts`, `supp_settings`. Go kit: `taxid`. Plan:
[PLAN-MODUL-PELANGGAN-PEMASOK](../../docs/plans/PLAN-MODUL-PELANGGAN-PEMASOK.md). Its sibling for
the selling side is `customers`; the two share no code but the kits.

**What it keeps.**
- Identity: code, name, legal name, party type (company, individual, government), foreign or
  not with a country.
- Tax identity as Coretax names it: `npwp` (16 digits; a 15-digit old NPWP is stored with a
  leading `0`), `nik`, `passport`, or `none`. Also PKP: input VAT is creditable only from a PKP.
- Addresses, each with an optional NITKU.
- Bank accounts where the supplier is paid.
- Contacts, payment days (empty = cash on delivery), and on hold with a reason.

**What it does not.** Payables, payments, withholding (PPh 23, 4(2), 21, 26), and the supplier's
tax certificates (SKB, Surat Keterangan PP 55 / PP 20/2026, DGT form) belong to the purchasing,
accounting, and tax modules, which read suppliers through their ports. `TaxIDKind` and
`PartyType` in the contract are what a withholding module decides rates from. Each company
keeps its own suppliers.

**Rules.**
- Codes are typed or generated per company (`PMS-00001`; the prefix is set on the list page).
- A tax ID another supplier of the company already has is a 409 listing them; saving again
  with `confirm_duplicate` keeps it.
- Suppliers, addresses, and bank accounts are **deactivated, never deleted**: documents and
  payments point at them. Contacts are deleted.
- **Bank accounts are guarded.** Adding, changing, or deactivating one needs
  `suppliers.sensitive` across the company — where money goes is changed only by those who may
  see it — and each change is a `security` audit record with before and after (number masked).
  An account held by someone other than the supplier saves with a warning: a changed account is
  the usual way a payment is diverted.
- **Personal data (UU PDP).** NIK, passport, and account numbers come masked in every list and
  detail. `GET /{id}/reveal` returns them whole to holders of `suppliers.sensitive` across the
  company and writes an `access` audit record.

## How another module uses it

Declare a port with the calls you need and map it to `contracts.SupplierClient` in an adapter:

| Call | When |
|---|---|
| `Usable(ctx, id)` | Inside your posting's `d.Tx`, before posting. `ErrNotFound`, `ErrInactive`, `ErrOnHold`. Holds the supplier's row shared until the transaction ends |
| `Snapshot(ctx, id, addressID)` | When posting: copy the name, tax ID (whole), PKP, and the address (0 = default billing) |
| `DefaultBankAccount(ctx, id)` | When paying: the active default account, whole. Writes an `access` record |
| `Get(ctx, id)` | Show a supplier, active or not; party type, tax ID kind, PKP for tax rules |
| `Search(ctx, q, limit)` | A picker; active suppliers only |

Store `supplier_id` as a plain ID. A web page of yours may call
`GET /api/v1/suppliers/lookup?q=` for a picker.

## Permissions

| Permission | For |
|---|---|
| `suppliers.read` | See suppliers (NIK, passport, account numbers masked) |
| `suppliers.manage` | Add, change, hold, deactivate; addresses, contacts, payment days; the code prefix (across the company) |
| `suppliers.sensitive` | See whole NIK, passport, and account numbers, and change bank accounts — across the company; audited |

## Pages

- **Pemasok** (`/suppliers`, nav "Pembelian", order 290): search, filter by status, add, the
  code prefix.
- **Detail** (`/suppliers/:id`): identity and tax, addresses, bank accounts, contacts, terms;
  change, hold or release, deactivate or activate.

## Changes

- **0.1.0** — first version: suppliers with tax identity, addresses with NITKU, guarded bank
  accounts, contacts, payment days, hold, generated codes, masked personal data with audited
  reveal, `SupplierClient`.
