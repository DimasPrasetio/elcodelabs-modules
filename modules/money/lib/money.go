// Package money is exact arithmetic for Rupiah amounts, quantities, and rates.
//
// Amount (2 decimals, DECIMAL(19,2)), Qty (4 decimals, DECIMAL(19,4)), and Rate (up to 10
// decimals, DECIMAL(19,10)) are distinct types, so a quantity is never added to a price by
// mistake. Addition and subtraction are exact; multiplication and division are computed exactly
// and rounded once, at the end, with the Rounding the caller chooses. Nothing here panics except
// the Must constructors: a value that does not fit its column returns ErrOverflow. Values cross
// the database and JSON as decimal strings ("1234567.89") — never as floats.
package money

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/govalues/decimal"
)

// Scales of the three types: what their MySQL columns hold.
const (
	AmountScale  = 2  // DECIMAL(19,2): sen
	QtyScale     = 4  // DECIMAL(19,4)
	RateMaxScale = 10 // DECIMAL(19,10)
)

// Errors every parse and operation may return; check them with errors.Is.
var (
	ErrSyntax   = errors.New("money: not a decimal number")
	ErrScale    = errors.New("money: too many decimal places")
	ErrOverflow = errors.New("money: value too large")
	ErrDivZero  = errors.New("money: division by zero")
	ErrWeights  = errors.New("money: weights must be zero or more, and not all zero")
	ErrRounding = errors.New("money: unknown rounding")
)

// Rounding says how a result is brought to its scale.
type Rounding int

const (
	// HalfUp rounds a half away from zero (1.005 → 1.01, −1.005 → −1.01): commercial rounding,
	// and the zero value.
	HalfUp Rounding = iota
	// HalfEven rounds a half to the even neighbour (1.005 → 1.00, 1.015 → 1.02).
	HalfEven
	// Up rounds away from zero (1.001 → 1.01).
	Up
	// Down rounds toward zero (1.009 → 1.00).
	Down
)

// --- exact core --------------------------------------------------------------------------

// maxCoef is the largest coefficient of a DECIMAL(19,s) column: nineteen nines.
var maxCoef = new(big.Int).SetUint64(9_999_999_999_999_999_999)

func pow10(n int) *big.Int { return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil) }

// rat is a decimal as an exact fraction.
func rat(d decimal.Decimal) *big.Rat {
	num := new(big.Int).SetUint64(d.Coef())
	if d.IsNeg() {
		num.Neg(num)
	}
	return new(big.Rat).SetFrac(num, pow10(d.Scale()))
}

// round brings the exact value x to exactly scale decimals with r — ErrOverflow when the result
// has more than 19 digits.
func round(x *big.Rat, scale int, r Rounding) (decimal.Decimal, error) {
	num := new(big.Int).Mul(x.Num(), pow10(scale))
	den := x.Denom() // always positive
	neg := num.Sign() < 0
	num.Abs(num)
	q, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	if r < HalfUp || r > Down {
		return decimal.Decimal{}, ErrRounding
	}
	if rem.Sign() != 0 {
		half := new(big.Int).Lsh(rem, 1).Cmp(den) // <0 below a half, 0 exactly a half, >0 above
		up := false
		switch r {
		case HalfUp:
			up = half >= 0
		case HalfEven:
			up = half > 0 || (half == 0 && q.Bit(0) == 1)
		case Up:
			up = true
		case Down:
			up = false
		default:
			return decimal.Decimal{}, ErrRounding
		}
		if up {
			q.Add(q, big.NewInt(1))
		}
	}
	if q.Cmp(maxCoef) > 0 {
		return decimal.Decimal{}, ErrOverflow
	}
	s := q.String()
	if len(s) <= scale {
		s = strings.Repeat("0", scale-len(s)+1) + s
	}
	if scale > 0 {
		s = s[:len(s)-scale] + "." + s[len(s)-scale:]
	}
	if neg && q.Sign() != 0 {
		s = "-" + s
	}
	d, err := decimal.ParseExact(s, scale)
	if err != nil {
		return decimal.Decimal{}, ErrOverflow
	}
	return d, nil
}

// parse reads s exactly, refusing more than maxScale decimals (trailing zeros aside) and more
// than maxInt whole digits (leading zeros aside) — checked on the text, before the decimal
// library could round a 20th digit away.
func parse(s string, maxScale, maxInt int) (decimal.Decimal, error) {
	if !plain(s) {
		return decimal.Decimal{}, fmt.Errorf("%w: %q", ErrSyntax, s)
	}
	whole, frac, _ := strings.Cut(strings.TrimPrefix(s, "-"), ".")
	whole, frac = strings.TrimLeft(whole, "0"), strings.TrimRight(frac, "0")
	if len(frac) > maxScale {
		return decimal.Decimal{}, fmt.Errorf("%w: %q has more than %d", ErrScale, s, maxScale)
	}
	if len(whole) > maxInt {
		return decimal.Decimal{}, fmt.Errorf("%w: %q", ErrOverflow, s)
	}
	// The digits that matter only: "007.500" is "7.5", however many zeros pad it.
	canon := whole
	if canon == "" {
		canon = "0"
	}
	if frac != "" {
		canon += "." + frac
	}
	if strings.HasPrefix(s, "-") {
		canon = "-" + canon
	}
	d, err := decimal.Parse(canon)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("%w: %q", ErrSyntax, s)
	}
	return d, nil
}

// plain is "-123.45": an optional minus, digits, and optionally a dot and digits — no plus,
// exponent, separators, or spaces.
func plain(s string) bool {
	s = strings.TrimPrefix(s, "-")
	whole, frac, dot := strings.Cut(s, ".")
	digits := func(p string) bool {
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
		return p != ""
	}
	return digits(whole) && (!dot || digits(frac))
}

// fixed is d at exactly scale decimals; the zero Decimal (scale 0) becomes 0.00….
func fixed(d decimal.Decimal, scale int) decimal.Decimal {
	if d.Scale() == scale {
		return d
	}
	return d.Pad(scale)
}

// --- Amount -------------------------------------------------------------------------------

// Amount is a sum of money in Rupiah, to the sen. The zero value is Rp0,00.
type Amount struct{ d decimal.Decimal }

// ParseAmount reads "1234567.89" or "-50" (a dot, no thousands separators, at most 2 decimals).
func ParseAmount(s string) (Amount, error) {
	d, err := parse(s, AmountScale, 19-AmountScale)
	if err != nil {
		return Amount{}, err
	}
	return Amount{fixed(d, AmountScale)}, nil
}

// MustAmount is ParseAmount for constants; it panics on a bad literal.
func MustAmount(s string) Amount {
	a, err := ParseAmount(s)
	if err != nil {
		panic(err)
	}
	return a
}

// Rupiah is n whole Rupiah — ErrOverflow beyond 17 digits.
func Rupiah(n int64) (Amount, error) { return amountOf(new(big.Rat).SetInt64(n), HalfUp) }

func amountOf(x *big.Rat, r Rounding) (Amount, error) {
	d, err := round(x, AmountScale, r)
	return Amount{d}, err
}

func (a Amount) dec() decimal.Decimal { return fixed(a.d, AmountScale) }

// String is the canonical form for APIs and the database: "1234567.89", "-0.50", "0.00".
func (a Amount) String() string { return a.dec().String() }

// Add is a + b.
func (a Amount) Add(b Amount) (Amount, error) {
	return amountOf(new(big.Rat).Add(rat(a.dec()), rat(b.dec())), HalfUp)
}

// Sub is a − b.
func (a Amount) Sub(b Amount) (Amount, error) {
	return amountOf(new(big.Rat).Sub(rat(a.dec()), rat(b.dec())), HalfUp)
}

func (a Amount) Neg() Amount         { return Amount{a.dec().Neg()} }
func (a Amount) Abs() Amount         { return Amount{a.dec().Abs()} }
func (a Amount) Sign() int           { return a.dec().Sign() }
func (a Amount) IsZero() bool        { return a.dec().IsZero() }
func (a Amount) Cmp(b Amount) int    { return a.dec().Cmp(b.dec()) }
func (a Amount) Equal(b Amount) bool { return a.Cmp(b) == 0 }

// Sum adds amounts; no amounts is zero.
func Sum(amounts ...Amount) (Amount, error) {
	total := new(big.Rat)
	for _, a := range amounts {
		total.Add(total, rat(a.dec()))
	}
	return amountOf(total, HalfUp)
}

// MulQty is a unit price times a quantity, rounded once to the sen.
func (a Amount) MulQty(q Qty, r Rounding) (Amount, error) {
	return amountOf(new(big.Rat).Mul(rat(a.dec()), rat(q.dec())), r)
}

// MulRate is an amount times a rate (a tax rate, a discount), rounded once to the sen.
func (a Amount) MulRate(x Rate, r Rounding) (Amount, error) {
	return amountOf(new(big.Rat).Mul(rat(a.dec()), rat(x.d)), r)
}

// MulFrac is an amount times num/den, computed exactly and rounded once — e.g. a DPP of 11/12
// of the price (PMK 131/2024): 11/12 × 10000000.00 = 9166666.666… → 9166666.67 half-up.
func (a Amount) MulFrac(num, den int64, r Rounding) (Amount, error) {
	if den == 0 {
		return Amount{}, ErrDivZero
	}
	return amountOf(new(big.Rat).Mul(rat(a.dec()), big.NewRat(num, den)), r)
}

// QuoQty is an amount divided by a quantity — a unit price from a line total — rounded once.
func (a Amount) QuoQty(q Qty, r Rounding) (Amount, error) {
	if q.IsZero() {
		return Amount{}, ErrDivZero
	}
	return amountOf(new(big.Rat).Quo(rat(a.dec()), rat(q.dec())), r)
}

// Round rounds to scale decimals (0 = whole Rupiah, 1 = ten sen) and stays an Amount:
// Round(0, HalfUp) of 1234.50 is 1235.00.
func (a Amount) Round(scale int, r Rounding) (Amount, error) {
	if scale < 0 || scale > AmountScale {
		return Amount{}, ErrScale
	}
	d, err := round(rat(a.dec()), scale, r)
	if err != nil {
		return Amount{}, err
	}
	// Back to 2 decimals in 19 digits: 99999999999999999.50 rounds to 18 whole digits, which
	// an Amount cannot hold.
	return amountOf(rat(d), HalfUp)
}

// --- Qty ----------------------------------------------------------------------------------

// Qty is a quantity to 4 decimals: 2.5 kg, 0.125 m³. The zero value is 0.0000.
type Qty struct{ d decimal.Decimal }

// ParseQty reads "2.5" (a dot, at most 4 decimals).
func ParseQty(s string) (Qty, error) {
	d, err := parse(s, QtyScale, 19-QtyScale)
	if err != nil {
		return Qty{}, err
	}
	return Qty{fixed(d, QtyScale)}, nil
}

// MustQty is ParseQty for constants; it panics on a bad literal.
func MustQty(s string) Qty {
	q, err := ParseQty(s)
	if err != nil {
		panic(err)
	}
	return q
}

// Units is n whole units.
func Units(n int64) (Qty, error) { return qtyOf(new(big.Rat).SetInt64(n), HalfUp) }

func qtyOf(x *big.Rat, r Rounding) (Qty, error) {
	d, err := round(x, QtyScale, r)
	return Qty{d}, err
}

func (q Qty) dec() decimal.Decimal { return fixed(q.d, QtyScale) }

// String is the canonical form: "2.5000".
func (q Qty) String() string { return q.dec().String() }

// Add is q + o.
func (q Qty) Add(o Qty) (Qty, error) {
	return qtyOf(new(big.Rat).Add(rat(q.dec()), rat(o.dec())), HalfUp)
}

// Sub is q − o.
func (q Qty) Sub(o Qty) (Qty, error) {
	return qtyOf(new(big.Rat).Sub(rat(q.dec()), rat(o.dec())), HalfUp)
}

// MulRate is a quantity times a rate (a conversion between units), rounded once.
func (q Qty) MulRate(x Rate, r Rounding) (Qty, error) {
	return qtyOf(new(big.Rat).Mul(rat(q.dec()), rat(x.d)), r)
}

func (q Qty) Neg() Qty         { return Qty{q.dec().Neg()} }
func (q Qty) Abs() Qty         { return Qty{q.dec().Abs()} }
func (q Qty) Sign() int        { return q.dec().Sign() }
func (q Qty) IsZero() bool     { return q.dec().IsZero() }
func (q Qty) Cmp(o Qty) int    { return q.dec().Cmp(o.dec()) }
func (q Qty) Equal(o Qty) bool { return q.Cmp(o) == 0 }

// --- Rate ---------------------------------------------------------------------------------

// Rate is a multiplier to up to 10 decimals and 9 whole digits: 0.12 for 12 %, a unit factor.
// The zero value is 0.
type Rate struct{ d decimal.Decimal }

// ParseRate reads "0.12" (a dot, at most 10 decimals).
func ParseRate(s string) (Rate, error) {
	d, err := parse(s, RateMaxScale, 19-RateMaxScale)
	return Rate{d}, err
}

// MustRate is ParseRate for constants; it panics on a bad literal.
func MustRate(s string) Rate {
	x, err := ParseRate(s)
	if err != nil {
		panic(err)
	}
	return x
}

// Percent is p % as a rate: Percent("12") is 0.12, Percent("2.5") is 0.025.
func Percent(p string) (Rate, error) {
	d, err := parse(p, RateMaxScale-2, 19-RateMaxScale+2)
	if err != nil {
		return Rate{}, err
	}
	x, err := round(new(big.Rat).Quo(rat(d), big.NewRat(100, 1)), RateMaxScale, HalfUp)
	if err != nil {
		return Rate{}, err
	}
	return ParseRate(x.String()) // trims, and checks the whole digits
}

// String is the rate without trailing zeros: "0.12".
func (x Rate) String() string { return x.d.Trim(0).String() }

func (x Rate) IsZero() bool      { return x.d.IsZero() }
func (x Rate) Cmp(o Rate) int    { return x.d.Cmp(o.d) }
func (x Rate) Equal(o Rate) bool { return x.Cmp(o) == 0 }

// --- allocation ---------------------------------------------------------------------------

// Weight is what Allocate divides by: an Amount or a Qty.
type Weight interface{ weight() *big.Rat }

func (a Amount) weight() *big.Rat { return rat(a.dec()) }
func (q Qty) weight() *big.Rat    { return rat(q.dec()) }

// Allocate splits total across lines in proportion to weights — an invoice discount or a
// shipping cost over its lines — so that the parts always add up to total, to the sen. Each
// part is first rounded toward zero; the sen left over then go one each to the parts that lost
// the most (the earlier line on a tie). A negative total is split the same way, all parts
// negative. Weights must be zero or more and not all zero; a zero weight gets nothing.
func Allocate[W Weight](total Amount, weights []W) ([]Amount, error) {
	sum := new(big.Rat)
	ws := make([]*big.Rat, len(weights))
	for i, w := range weights {
		ws[i] = w.weight()
		if ws[i].Sign() < 0 {
			return nil, ErrWeights
		}
		sum.Add(sum, ws[i])
	}
	if sum.Sign() == 0 {
		return nil, ErrWeights
	}
	sen := new(big.Int).SetUint64(total.dec().Coef()) // |total| in sen
	parts := make([]*big.Int, len(ws))
	rems := make([]*big.Rat, len(ws))
	given := new(big.Int)
	for i, w := range ws {
		share := new(big.Rat).Quo(new(big.Rat).Mul(new(big.Rat).SetInt(sen), w), sum)
		parts[i] = new(big.Int).Quo(share.Num(), share.Denom())
		rems[i] = new(big.Rat).Sub(share, new(big.Rat).SetInt(parts[i]))
		given.Add(given, parts[i])
	}
	order := make([]int, len(ws))
	for i := range order {
		order[i] = i
	}
	// Largest remainder first, the earlier line on a tie.
	sort.SliceStable(order, func(i, j int) bool { return rems[order[i]].Cmp(rems[order[j]]) > 0 })
	left := new(big.Int).Sub(sen, given).Int64() // fewer than len(ws): each part lost under 1 sen
	for k := int64(0); k < left; k++ {
		parts[order[k]].Add(parts[order[k]], big.NewInt(1))
	}
	out := make([]Amount, len(ws))
	for i, p := range parts {
		x := new(big.Rat).SetFrac(p, big.NewInt(100))
		if total.Sign() < 0 {
			x.Neg(x)
		}
		a, err := amountOf(x, HalfUp)
		if err != nil {
			return nil, err
		}
		out[i] = a
	}
	return out, nil
}
