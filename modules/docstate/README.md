# docstate

Go kit: the life of a business document. Installed into `apps/api/internal/shared/docstate`; a
module that uses it lists `"go_kits": ["docstate"]`. Standard library only.

```
draft ──post──▶ posted ──cancel (reason)──▶ cancelled ──amend──▶ a new draft
  └── edit, delete
```

| From | Action | To | Checked here |
|---|---|---|---|
| `draft` | `Edit` | `draft` | — |
| `draft` | `Post` | `posted` | — (the module checks the period and takes the number) |
| `draft` | `Delete` | gone | — (the module checks nothing points at it) |
| `posted` | `Cancel` | `cancelled` | `CancelReason`: 5–500 characters |
| `cancelled` | `Amend` | stays `cancelled`; a new `draft` corrects it | — |
| anything else | | | `*TransitionError` (`errors.Is(err, docstate.ErrTransition)`), with an Indonesian message saying what to do instead |

A posted document is never edited. It is cancelled, and its correction is a new draft with
`AmendedFrom` = the cancelled one and the number `docstate.Amendment(number, revision)` gives:
`INV/2026/10/0007` → `INV/2026/10/0007-1` → `…-2`. The amendment's number comes from the original,
so it takes nothing from the numbering series.

## In a module

- Embed `docstate.Fields` in the domain type and add its columns to the table (the comment on
  `Fields` has them, with `UNIQUE KEY (company_id, amended_from)` so a cancelled document is
  amended once).
- Map the errors in presentation through `shared/apperror`: `ErrTransition` and `ErrStale` → 409
  with the error's message, `ErrReason` → 422.
- Every transition is a **guarded UPDATE** — the old status in the `WHERE` — and
  `docstate.Guard(rowsAffected)` turns "no row changed" into `ErrStale`:

  ```sql
  -- name: MarkPosted :execrows
  UPDATE inv_invoices SET status = 'posted', number = ?, posted_at = ?, posted_by = ?
  WHERE id = ? AND company_id = ? AND status = 'draft';
  ```

## Posting recipe

One `d.Tx.Do`, so all of it happens or none of it does — a rollback also gives the number back:

1. Lock the document: `SELECT … WHERE id = ? AND company_id = ? FOR UPDATE`, then
   `docstate.Check(doc.Status, docstate.Post)`.
2. `periods.EnsureOpen(ctx, "<area>", doc.Date)` — the `period` module, through a port — **before
   any other row lock** (stock, balances): while someone moves a lock date, postings queue behind
   that change, and one already holding another row could deadlock (`d.Tx` retries it, but
   needlessly).
3. The document's own work: stock, balances, lines.
4. `number, err := numbering.Next(ctx, "<module>.<doc>", doc.Date)` — the `numbering` module,
   through a port — **late**: the series' counter stays locked until the commit, so other
   postings of the series wait for whatever comes after it. An amendment keeps the number it
   already has.
5. The guarded `UPDATE` above, then `docstate.Guard(n)`.
6. `d.Events.Publish(ctx, "<module>.<doc>.posted", 1, …)` — stock, journals, and others react
   after the commit.

Cancel is the same shape: lock, `Check(Cancel)`, `CancelReason`, `EnsureOpen` for the
**document's** date (closed books do not change), guarded `UPDATE … AND status = 'posted'`, and a
`….cancelled` event so the modules that reacted to the posting reverse it.

## Changes

- **0.1.0** — first version: statuses, actions, transition table, `CancelReason`, `Guard`,
  `Fields`, `AmendedNumber`/`Amendment`.
