# fixinvoice

Testbed fixture: an invoice module built on `money`, `docstate`, `numbering`, and `period` — the
posting recipe of the `docstate` README, end to end. `?hold_ms=` on post keeps the transaction
open (the testbed's lock-race check); `?fail_after_number=1` fails it right after the number is
taken (the rollback check). With the optional `Customers` port wired, an invoice's `customer_id`
is checked (`Usable`) and copied (`Snapshot`) inside the posting's transaction.

## Changes

- **0.2.0** — an optional customer: port `Customers` (adapter to `customers`), checked and copied
  when posting.
- **0.1.0** — first version.
