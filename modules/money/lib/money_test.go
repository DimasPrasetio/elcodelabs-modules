package money

import (
	"encoding/json"
	"errors"
	"math/big"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// TestRounding is the core every operation rounds through, at the edges of a half.
func TestRounding(t *testing.T) {
	cases := []struct {
		num, den int64 // the exact value num/den
		r        Rounding
		want     string
	}{
		{1005, 1000, HalfUp, "1.01"},
		{1005, 1000, HalfEven, "1.00"},
		{1005, 1000, Up, "1.01"},
		{1005, 1000, Down, "1.00"},
		{-1005, 1000, HalfUp, "-1.01"},
		{-1005, 1000, HalfEven, "-1.00"},
		{-1005, 1000, Up, "-1.01"},
		{-1005, 1000, Down, "-1.00"},
		{1015, 1000, HalfEven, "1.02"},
		{1025, 1000, HalfEven, "1.02"},
		{100499999, 100000000, HalfUp, "1.00"}, // 1.00499999
		{100500001, 100000000, HalfEven, "1.01"},
		{1001, 1000, Up, "1.01"},
		{1009, 1000, Down, "1.00"},
		{-1, 1000, HalfUp, "0.00"}, // no "-0.00"
		{-5, 1000, HalfUp, "-0.01"},
		{-5, 1000, HalfEven, "0.00"},
		{2, 3, HalfUp, "0.67"},
		{-2, 3, Down, "-0.66"},
		{0, 1, Up, "0.00"},
	}
	for _, c := range cases {
		d, err := round(ratOf(c.num, c.den), 2, c.r)
		if err != nil || d.String() != c.want {
			t.Errorf("round(%d/%d, %d) = %s, %v; want %s", c.num, c.den, c.r, d, err, c.want)
		}
	}
}

func TestMulQty(t *testing.T) {
	got, err := MustAmount("0.67").MulQty(MustQty("1.5"), HalfUp) // 1.005
	if err != nil || got.String() != "1.01" {
		t.Errorf("0.67 × 1.5 = %s, %v", got, err)
	}
	got, _ = MustAmount("0.67").MulQty(MustQty("1.5"), HalfEven)
	if got.String() != "1.00" {
		t.Errorf("0.67 × 1.5 half-even = %s", got)
	}
	got, _ = MustAmount("12500").MulQty(MustQty("2.125"), Down)
	if got.String() != "26562.50" {
		t.Errorf("12500 × 2.125 = %s", got)
	}
}

func ratOf(num, den int64) *big.Rat { return big.NewRat(num, den) }

func mulRat(xs ...*big.Rat) *big.Rat {
	out := big.NewRat(1, 1)
	for _, x := range xs {
		out.Mul(out, x)
	}
	return out
}

func TestRoundToRupiah(t *testing.T) {
	for in, want := range map[string]string{"1234.50": "1235.00", "1234.49": "1234.00", "-1234.50": "-1235.00", "0.50": "1.00", "-0.49": "0.00"} {
		got, err := MustAmount(in).Round(0, HalfUp)
		if err != nil || got.String() != want {
			t.Errorf("Round(%s) = %s, %v; want %s", in, got, err, want)
		}
	}
	if got, _ := MustAmount("1234.55").Round(1, HalfEven); got.String() != "1234.60" {
		t.Errorf("Round(1, HalfEven) of 1234.55 = %s", got)
	}
	if _, err := MustAmount("1").Round(3, HalfUp); !errors.Is(err, ErrScale) {
		t.Errorf("Round(3) = %v; want ErrScale", err)
	}
}

func TestMulFracDPP(t *testing.T) {
	dpp, err := MustAmount("10000000").MulFrac(11, 12, HalfUp)
	if err != nil || dpp.String() != "9166666.67" {
		t.Fatalf("11/12 × 10.000.000 = %s, %v", dpp, err)
	}
	ppn, err := dpp.MulRate(MustRate("0.12"), HalfUp)
	if err != nil || ppn.String() != "1100000.00" {
		t.Fatalf("12%% × %s = %s, %v", dpp, ppn, err)
	}
	if _, err := dpp.MulFrac(1, 0, HalfUp); !errors.Is(err, ErrDivZero) {
		t.Errorf("MulFrac(1, 0) = %v", err)
	}
}

func TestQuoQty(t *testing.T) {
	got, err := MustAmount("100").QuoQty(MustQty("3"), HalfUp)
	if err != nil || got.String() != "33.33" {
		t.Errorf("100 / 3 = %s, %v", got, err)
	}
	if _, err := MustAmount("1").QuoQty(Qty{}, HalfUp); !errors.Is(err, ErrDivZero) {
		t.Errorf("1 / 0 = %v", err)
	}
}

func TestParse(t *testing.T) {
	ok := map[string]string{"0": "0.00", "1": "1.00", "1.5": "1.50", "1.500": "1.50", "-0.01": "-0.01", "-0": "0.00", "007.10": "7.10", "1." + strings.Repeat("0", 400): "1.00", strings.Repeat("0", 400) + "1": "1.00", "99999999999999999.99": "99999999999999999.99"}
	for in, want := range ok {
		a, err := ParseAmount(in)
		if err != nil || a.String() != want {
			t.Errorf("ParseAmount(%q) = %s, %v; want %s", in, a, err, want)
		}
	}
	bad := map[string]error{"1.005": ErrScale, "1,5": ErrSyntax, "": ErrSyntax, "Rp1": ErrSyntax, "1e3": ErrSyntax, "+1": ErrSyntax, "1.": ErrSyntax, ".5": ErrSyntax, "1 000": ErrSyntax, "--1": ErrSyntax, " 12": ErrSyntax, "1\n": ErrSyntax,
		"1.0000000000000000001": ErrScale, "1.009999999999999999999": ErrScale, "0.0000000000000000000001": ErrScale, "99999999999999999.994999999": ErrScale, "100000000000000000": ErrOverflow, "99999999999999999999": ErrOverflow}
	for in, want := range bad {
		_, err := ParseAmount(in)
		if err == nil || (want != nil && !errors.Is(err, want)) {
			t.Errorf("ParseAmount(%q) = %v; want %v", in, err, want)
		}
	}
	if q, err := ParseQty("2.125"); err != nil || q.String() != "2.1250" {
		t.Errorf("ParseQty(2.125) = %s, %v", q, err)
	}
	if _, err := ParseQty("0.00001"); !errors.Is(err, ErrScale) {
		t.Errorf("ParseQty(0.00001) = %v", err)
	}
	if x, err := Percent("12"); err != nil || x.String() != "0.12" {
		t.Errorf("Percent(12) = %s, %v", x, err)
	}
	if x, err := Percent("2.5"); err != nil || x.String() != "0.025" {
		t.Errorf("Percent(2.5) = %s, %v", x, err)
	}
	if _, err := ParseRate("1000000000"); !errors.Is(err, ErrOverflow) {
		t.Errorf("ParseRate(1e9) = %v; want ErrOverflow", err)
	}
}

func TestOverflow(t *testing.T) {
	max := MustAmount("99999999999999999.99")
	if _, err := max.Add(MustAmount("0.01")); !errors.Is(err, ErrOverflow) {
		t.Errorf("max + 0.01 = %v; want ErrOverflow", err)
	}
	if _, err := max.MulQty(MustQty("2"), HalfUp); !errors.Is(err, ErrOverflow) {
		t.Errorf("max × 2 = %v; want ErrOverflow", err)
	}
	if _, err := Rupiah(1 << 62); !errors.Is(err, ErrOverflow) {
		t.Errorf("Rupiah(2^62) = %v; want ErrOverflow", err)
	}
	if got, err := max.Neg().Sub(MustAmount("0")); err != nil || got.String() != "-99999999999999999.99" {
		t.Errorf("-max = %s, %v", got, err)
	}
}

func TestRoundNearTheMaximum(t *testing.T) {
	for _, c := range []struct {
		in    string
		scale int
	}{{"99999999999999999.50", 0}, {"99999999999999999.95", 1}, {"-99999999999999999.50", 0}} {
		if got, err := MustAmount(c.in).Round(c.scale, HalfUp); !errors.Is(err, ErrOverflow) {
			t.Errorf("Round(%s, %d) = %s, %v; want ErrOverflow", c.in, c.scale, got, err)
		}
	}
	if got, err := MustAmount("99999999999999999.49").Round(0, HalfUp); err != nil || got.String() != "99999999999999999.00" {
		t.Errorf("Round just below = %s, %v", got, err)
	}
	if _, err := MustAmount("1").MulQty(MustQty("1.5"), Rounding(7)); !errors.Is(err, ErrRounding) {
		t.Errorf("unknown rounding = %v", err)
	}
	if _, err := ParseQty("0.99999999999999999999"); !errors.Is(err, ErrScale) {
		t.Errorf("ParseQty 20 digits = %v", err)
	}
	if _, err := ParseRate("0.12000000000000000001"); !errors.Is(err, ErrScale) {
		t.Errorf("ParseRate 20 digits = %v", err)
	}
	if _, err := Percent("12.0000000000000000000001"); !errors.Is(err, ErrScale) {
		t.Errorf("Percent 22 digits = %v", err)
	}
}

func TestAllocateManyLines(t *testing.T) {
	ws := make([]Qty, 20000)
	for i := range ws {
		ws[i], _ = Units(int64(i%7 + 1))
	}
	start := time.Now()
	parts, err := Allocate(MustAmount("1000000.01"), ws)
	if err != nil || time.Since(start) > 2*time.Second {
		t.Fatalf("20000 lines: %v, %s", err, time.Since(start))
	}
	if sum, _ := Sum(parts...); sum.String() != "1000000.01" {
		t.Errorf("sum = %s", sum)
	}
}

func TestZeroValues(t *testing.T) {
	var a Amount
	var q Qty
	var x Rate
	if a.String() != "0.00" || q.String() != "0.0000" || x.String() != "0" || !a.IsZero() {
		t.Errorf("zero values: %s %s %s", a, q, x)
	}
	if s, err := Sum(); err != nil || !s.IsZero() {
		t.Errorf("Sum() = %s, %v", s, err)
	}
	if b, err := a.Add(MustAmount("1.5")); err != nil || b.String() != "1.50" {
		t.Errorf("0 + 1.5 = %s, %v", b, err)
	}
}

func TestDistinctTypesCompareAndSum(t *testing.T) {
	s, err := Sum(MustAmount("0.10"), MustAmount("0.20"), MustAmount("-0.05"))
	if err != nil || s.String() != "0.25" {
		t.Errorf("Sum = %s, %v", s, err)
	}
	if MustAmount("1").Cmp(MustAmount("1.00")) != 0 || !MustAmount("2").Equal(MustAmount("2.0")) {
		t.Error("1 and 1.00 differ")
	}
	if MustAmount("-3").Abs().String() != "3.00" || MustAmount("3").Neg().String() != "-3.00" {
		t.Error("Abs/Neg")
	}
}

func TestAllocate(t *testing.T) {
	parts, err := Allocate(MustAmount("100"), []Qty{MustQty("1"), MustQty("1"), MustQty("1")})
	if err != nil {
		t.Fatal(err)
	}
	if parts[0].String() != "33.34" || parts[1].String() != "33.33" || parts[2].String() != "33.33" {
		t.Errorf("100 / 3 = %v", parts)
	}
	parts, _ = Allocate(MustAmount("-0.05"), []Amount{MustAmount("1"), MustAmount("0"), MustAmount("3")})
	if parts[0].String() != "-0.01" || parts[1].String() != "0.00" || parts[2].String() != "-0.04" {
		t.Errorf("-0.05 by 1:0:3 = %v", parts)
	}
	if _, err := Allocate(MustAmount("1"), []Qty{MustQty("0")}); !errors.Is(err, ErrWeights) {
		t.Errorf("all-zero weights = %v", err)
	}
	if _, err := Allocate(MustAmount("1"), []Qty{MustQty("-1"), MustQty("2")}); !errors.Is(err, ErrWeights) {
		t.Errorf("negative weight = %v", err)
	}
	if _, err := Allocate(MustAmount("1"), []Qty{}); !errors.Is(err, ErrWeights) {
		t.Errorf("no weights = %v", err)
	}
}

// TestAllocateAddsUp is the property: whatever the weights, the parts add up to the total, and
// no part is more than one sen from its exact share.
func TestAllocateAddsUp(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 2000; i++ {
		n := 1 + rng.Intn(12)
		ws := make([]Amount, n)
		for j := range ws {
			ws[j], _ = Rupiah(int64(rng.Intn(5)) * int64(rng.Intn(1_000_000)))
		}
		ws[rng.Intn(n)], _ = Rupiah(1 + int64(rng.Intn(1000)))
		total, _ := amountOf(ratOf(rng.Int63n(1_000_000_000_00)-rng.Int63n(1_000_000_000_00), 100), HalfUp)
		parts, err := Allocate(total, ws)
		if err != nil {
			t.Fatal(err)
		}
		sum, _ := Sum(parts...)
		if !sum.Equal(total) {
			t.Fatalf("Allocate(%s, %v) = %v, adds up to %s", total, ws, parts, sum)
		}
		var wsum = ratOf(0, 1)
		for _, w := range ws {
			wsum.Add(wsum, rat(w.dec()))
		}
		for j, p := range parts {
			exact := mulRat(rat(total.dec()), rat(ws[j].dec()))
			exact.Quo(exact, wsum)
			diff := exact.Sub(exact, rat(p.dec()))
			if diff.Abs(diff).Cmp(ratOf(1, 100)) >= 0 {
				t.Fatalf("part %d of Allocate(%s, %v) = %s, a sen or more from its share", j, total, ws, p)
			}
		}
	}
}

func TestCodec(t *testing.T) {
	var a Amount
	for _, src := range []any{"1234567.89", []byte("1234567.89")} {
		if err := a.Scan(src); err != nil || a.String() != "1234567.89" {
			t.Errorf("Scan(%T) = %s, %v", src, a, err)
		}
	}
	if err := a.Scan(int64(5)); err != nil || a.String() != "5.00" {
		t.Errorf("Scan(int64) = %s, %v", a, err)
	}
	if err := a.Scan(1.5); err == nil {
		t.Error("Scan(float64) accepted")
	}
	if err := a.Scan(nil); err == nil {
		t.Error("Scan(nil) accepted")
	}
	if v, _ := MustAmount("-0.5").Value(); v != "-0.50" {
		t.Errorf("Value = %v", v)
	}
	var q Qty
	if err := q.Scan([]byte("2.5000")); err != nil || q.String() != "2.5000" {
		t.Errorf("Qty.Scan = %s, %v", q, err)
	}
	var x Rate
	if err := x.Scan("0.1200000000"); err != nil || x.String() != "0.12" {
		t.Errorf("Rate.Scan = %s, %v", x, err)
	}

	type line struct {
		Price Amount `json:"price"`
		Qty   Qty    `json:"qty"`
		Tax   Rate   `json:"tax"`
	}
	b, err := json.Marshal(line{MustAmount("1500.5"), MustQty("2.5"), MustRate("0.12")})
	if err != nil || string(b) != `{"price":"1500.50","qty":"2.5000","tax":"0.12"}` {
		t.Errorf("Marshal = %s, %v", b, err)
	}
	var back line
	if err := json.Unmarshal(b, &back); err != nil || back.Price.String() != "1500.50" || back.Qty.String() != "2.5000" || back.Tax.String() != "0.12" {
		t.Errorf("Unmarshal = %+v, %v", back, err)
	}
	// A bare JSON number is read exactly as written, never through a float.
	if err := json.Unmarshal([]byte(`{"price":0.1,"qty":3,"tax":0.11}`), &back); err != nil || back.Price.String() != "0.10" {
		t.Errorf("Unmarshal number = %+v, %v", back, err)
	}
	for _, bad := range []string{`{"price":"1.005"}`, `{"price":null}`, `{"price":"abc"}`, `{"price":true}`, `{"price":1e3}`} {
		var te *json.UnmarshalTypeError
		if err := json.Unmarshal([]byte(bad), &back); !errors.As(err, &te) || te.Field != "price" {
			t.Errorf("Unmarshal(%s) = %v; want a type error naming the field", bad, err)
		}
	}
}

func TestFormat(t *testing.T) {
	for in, want := range map[string]string{"1234567.89": "Rp1.234.567,89", "0": "Rp0,00", "-50000": "-Rp50.000,00", "999": "Rp999,00", "1000": "Rp1.000,00", "-0.05": "-Rp0,05"} {
		if got := Format(MustAmount(in)); got != want {
			t.Errorf("Format(%s) = %s; want %s", in, got, want)
		}
	}
	if got := FormatWhole(MustAmount("50000")); got != "Rp50.000" {
		t.Errorf("FormatWhole = %s", got)
	}
	if got := FormatWhole(MustAmount("1500.5")); got != "Rp1.500,50" {
		t.Errorf("FormatWhole = %s", got)
	}
	for in, want := range map[string]string{"2.5": "2,5", "1250": "1.250", "0": "0", "-0.125": "-0,125", "1234567.0001": "1.234.567,0001"} {
		if got := FormatQty(MustQty(in)); got != want {
			t.Errorf("FormatQty(%s) = %s; want %s", in, got, want)
		}
	}
}
