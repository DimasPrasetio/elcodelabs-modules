# money

Go kit: exact Rupiah amounts, quantities, and rates. Installed into
`apps/api/internal/shared/money`; a module that uses it lists `"go_kits": ["money"]`. Built on
[`govalues/decimal`](https://pkg.go.dev/github.com/govalues/decimal) (MIT), wrapped so the library
underneath can change without touching a module.

| Type | For | MySQL column | Decimals |
|---|---|---|---|
| `money.Amount` | Prices, totals, DPP, PPN, balances | `DECIMAL(19,2)` | 2 (sen) |
| `money.Qty` | 2.5 kg, 0.125 m³ | `DECIMAL(19,4)` | 4 |
| `money.Rate` | 0.12 (12 %), unit factors | `DECIMAL(19,10)` | up to 10 |

The three are distinct types: `Amount.Add(Amount)` compiles, `Amount.Add(Qty)` does not. Never a
`float64`, and never a `FLOAT`/`DOUBLE` column (the library's `exact-numbers` rule).

## Arithmetic

- `Add`, `Sub`, `Sum(...)` are exact. `Neg`, `Abs`, `Sign`, `IsZero`, `Cmp`, `Equal`.
- `price.MulQty(qty, r)`, `amount.MulRate(rate, r)`, `amount.MulFrac(11, 12, r)`,
  `total.QuoQty(qty, r)`: computed exactly, **rounded once** at the end with the rounding you
  pass — `money.HalfUp` (commercial: a half goes away from zero; the zero value), `HalfEven`, `Up`,
  `Down`. `amount.Round(0, money.HalfUp)` rounds to whole Rupiah.
- `money.Allocate(total, weights)` splits a total over lines in proportion to `[]Qty` or
  `[]Amount` weights (an invoice discount, shipping): the parts **always** add up to the total;
  leftover sen go to the lines with the largest remainders.
- Every operation that can exceed 19 digits returns `money.ErrOverflow` instead of panicking.
  Only `MustAmount`/`MustQty`/`MustRate` panic — for literals in code.

```go
price := money.MustAmount("10000000")
dpp, err := price.MulFrac(11, 12, money.HalfUp)    // 9166666.67 — DPP nilai lain (PMK 131/2024)
ppn, err := dpp.MulRate(money.MustRate("0.12"), money.HalfUp) // 1100000.00
```

Tax policy — when 11/12 applies, whether PPN rounds per line or per invoice — is not here; it
belongs to the module that owns tax, which uses these operations.

## Text, database, JSON

- `ParseAmount("1234567.89")` reads the canonical form: a minus, digits, a dot — no thousands
  separators, no exponent, at most 2 decimals (`"1.500"` is fine, `"1.505"` is `ErrScale`).
  `String()` writes it back: `"1234567.89"`. `ParseQty`, `ParseRate`, `Percent("12")` likewise.
- `money.Format(a)` → `"Rp1.234.567,89"`, `FormatWhole(a)` → `"Rp50.000"` when there are no sen,
  `FormatQty(q)` → `"2,5"` — for documents and messages, not APIs.
- **sqlc maps MySQL `DECIMAL` to `string`** (`sql.NullString` when nullable). Repositories parse
  on the way in and write `String()` on the way out:

  ```go
  total, err := money.ParseAmount(row.Total)
  err = q.InsertInvoice(ctx, sqlc.InsertInvoiceParams{Total: total.String(), …})
  ```

  The types also implement `sql.Scanner` and `driver.Valuer` for hand-written `Scan`s.
- JSON is always a **string** (`"price":"1500.50"`): a JSON number is a float in JavaScript. A bare
  number sent by a client is still read exactly as written. A bad value (`"1.005"`, `"abc"`,
  `null`) fails decoding with a `*json.UnmarshalTypeError` naming the field, so
  `request.DecodeJSON` answers 400 "Kolom lines.price berisi tipe data yang salah". An optional
  amount is a `*money.Amount`; a message per rule needs a `string` field and `ParseAmount`.

## Changes

- **0.1.0** — first version: `Amount`, `Qty`, `Rate`; half-up, half-even, up, down rounding;
  `MulQty`, `MulRate`, `MulFrac`, `QuoQty`, `Round`; `Allocate`; id-ID formatting; SQL and JSON.
