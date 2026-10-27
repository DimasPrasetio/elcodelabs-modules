# fixinvoice

Testbed fixture: an invoice module built on `money`, `docstate`, `numbering`, and `period` — the
posting recipe of the `docstate` README, end to end. `?hold_ms=` on post keeps the transaction
open (the testbed's lock-race check); `?fail_after_number=1` fails it right after the number is
taken (the rollback check).

## Changes

- **0.1.0** — first version.
