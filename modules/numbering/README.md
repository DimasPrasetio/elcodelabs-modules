# numbering

Numbers for business documents — `INV/BDG/2026/10/0007` — per company and, if chosen, per
branch, restarting each year, month, or day, **without gaps**. The page is
**Pengaturan › Penomoran** (`/numbering`): every document type the installed modules declare, its
format, and the next number of each series.

| Token | Prints |
|---|---|
| `{YYYY}` `{YY}` `{MM}` `{DD}` | The **document's** date (not the server's clock) |
| `{SEQ:n}` | The counter, zero-padded to n digits (1–12) — exactly one |
| `{BRANCH}` | The branch's code (the company's code for a company-wide series) |
| `{COMPANY}` | The company's code |

A format is refused when it could repeat a number:

- a yearly reset without the year, a monthly one without year and month, a daily one without
  year, month, and day; a per-branch series without `{BRANCH}`;
- a code (`{BRANCH}`, `{COMPANY}`) not set apart by `/`, `.`, `_`, or a space — `-` does not count,
  since codes may hold it (`A1`+`001` would read like `A`+`1001`);
- a pattern ending in a code: a code may end in `-1`, so a number could equal another's revision
  (`docstate`: `…-1`);
- a number that could pass 90 characters (codes counted at 20, the counter at 12 digits), so it
  and a revision suffix fit `VARCHAR(100)`.

Once a number of a type was taken in a company, its **reset and per-branch choice are fixed**
(409): a new series would start from 1 and print numbers already given. The pattern may still
change.

Codes come from the optional `Units` port (adapter for `organization`); without it they print
the company's and branch's IDs.

## Gapless

`Next` runs **inside the caller's `d.Tx`** (outside it returns `contracts.ErrNotInTx`):

```sql
INSERT INTO num_counters (…, last_number) VALUES (…, 1) ON DUPLICATE KEY UPDATE last_number = last_number + 1;
SELECT last_number FROM num_counters WHERE … FOR UPDATE;
```

The counter row stays locked until the posting commits or rolls back — a rollback gives the
number back, so the next posting gets it. Every number given is also kept in `num_issued`
(unique per company and type, rolled back with the posting): a value whose number was printed
before — by a branch that once had this code, say — or that reads like a revision of one
(`…-1` after a pattern change) is passed over, so a number is **never** printed twice. Those
passed-over values are the only gaps a series can show. Postings of the same series queue on that row for the
milliseconds a posting takes; per-branch series do not queue on each other. A number is taken
at **posting**, never for a draft.

## For a module that posts documents

```go
// ports/numbers.go — your module's own port; the adapter maps it to NumberingClient
type Numbers interface {
	Declare(docType, label string, def NumberFormat)
	Next(ctx context.Context, docType string, date kernel.Date) (string, error)
}
```

- In `New`: `numbers.Declare("sales.invoice", "Faktur penjualan", {Pattern: "INV/{YYYY}/{MM}/{SEQ:4}", Reset: "monthly"})`.
  A bad name or format panics at start.
- When posting, inside `d.Tx`, after `period.EnsureOpen` and the document's own work — as late
  as possible, since the series stays locked until the commit:
  `number, err := numbers.Next(ctx, "sales.invoice", doc.Date)` — the
  [`docstate` recipe](../docstate/README.md#posting-recipe).
- A per-branch series counts in the branch of the request's scope. A document of another branch
  passes it: `kernel.WithScope(ctx, kernel.Scope{CompanyID: sc.CompanyID, BranchID: doc.BranchID, Location: sc.Location})`.
- Keep the number in `VARCHAR(100)` with `UNIQUE KEY (company_id, number)`: a backstop that
  turns any duplicate into a failed posting instead of two documents with one number.
- An amendment (`INV/…/0007-1`) takes its number from the original through `docstate`, not from
  here.

## Moving from another system

**Nomor berikutnya** sets where a series continues in the current period — only upward (at most
999 999 999, leaving the series room up to 999 999 999 999), with a reason, needing `numbering.manage` in that branch (the whole company for a company-wide
series), recorded as a **security** audit event. Changing a format needs `numbering.manage`
across the company; numbers already given keep theirs.

Not here: tax invoice numbers (NSFP / e-Faktur) — Coretax issues those.

## Changes

- **0.1.0** — first version: declared document types, formats per company, gapless counters
  per company/branch/period, a record of every number given, previews, raising the next
  number, `NumberingClient`.
