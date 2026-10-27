# period

Closing the books, per company. A **lock date** closes that day and every day before it — for
every area (`*`) or for one area a module declares (`sales`, `purchasing`, `payroll`, …). A
document dated in a closed period is neither posted nor cancelled, so a month already reported
stays as reported. The page is **Pengaturan › Tutup buku** (`/period`).

| | Who | Rule |
|---|---|---|
| Close (a later date) | `period.manage` across the company | Before today; an area's lock never earlier than the company-wide one |
| Reopen (an earlier date, or none) | `period.unlock` across the company | A reason (5–500 characters), recorded in the audit log as a **security** event |
| Fiscal year | `period.manage` across the company | The first month (1–12), January by default; a **security** event (it moves what a fiscal-year closing covers) |

What closes an area is the **later** of its own lock and `*`. There is no lock nobody can
reopen (a hard lock) in this version.

## For a module that posts documents

Declare the port in your module (never import this one), and call it inside the `d.Tx` that
posts — the [`docstate` recipe](../docstate/README.md#posting-recipe):

```go
// ports/periods.go
type Periods interface {
	Declare(area, label string)
	EnsureOpen(ctx context.Context, area string, date kernel.Date) error // ErrPeriodClosed → 409
}
```

- `Declare("sales", "Penjualan")` in your `New`. `EnsureOpen` refuses an area nobody declared.
- `EnsureOpen(ctx, "sales", doc.Date)` when posting **and** when cancelling (the document's own
  date). It returns a `*contracts.ClosedError` (`errors.Is(err, contracts.ErrClosed)`) whose
  message names the lock date — the adapter maps it to your port's error and keeps the message.
- Inside `d.Tx` it holds the company's `period_settings` row **shared** until the transaction
  ends, and every lock change holds it **exclusive**: a lock moved while a posting runs waits
  for it, or the posting waits for the lock — never half of each. Outside a transaction it only
  reads (fine for a form's early warning, not for posting).
- Call it **early** — right after locking the document, before other row locks (stock,
  balances). While a lock change waits, new postings queue behind it; one that already holds a
  row another posting needs would deadlock, which `d.Tx` retries but costs a round.
- `FiscalYear(ctx, date)` is the first and last day of the company's fiscal year containing
  `date` — for reports and the ten-year retention counted from the end of a fiscal year
  (UU KUP Art. 28(11)).

## Changes

- **0.1.0** — first version: lock dates per company and area, reopening with `period.unlock`
  and a reason, the fiscal year's first month, `PeriodClient` (`Declare`, `EnsureOpen`,
  `FiscalYear`).
