package money

import "strings"

// Format is an amount as Indonesians read it: "Rp1.234.567,89", "-Rp50.000,00" (PUEBI: no
// space after Rp). Use it in documents and messages; APIs send String().
func Format(a Amount) string { return format(a, false) }

// FormatWhole is Format without ",00" when the amount is whole: "Rp50.000", "Rp1.500,50".
func FormatWhole(a Amount) string { return format(a, true) }

func format(a Amount, dropZeroSen bool) string {
	s := a.Abs().String() // "1234567.89"
	whole, sen, _ := strings.Cut(s, ".")
	var b strings.Builder
	if a.Sign() < 0 {
		b.WriteByte('-')
	}
	b.WriteString("Rp")
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if !(dropZeroSen && sen == "00") {
		b.WriteByte(',')
		b.WriteString(sen)
	}
	return b.String()
}

// FormatQty is a quantity with a decimal comma and only the decimals it needs: "2,5", "1.250".
func FormatQty(q Qty) string {
	s := strings.TrimRight(strings.TrimRight(q.Abs().String(), "0"), ".")
	whole, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	if q.Sign() < 0 {
		b.WriteByte('-')
	}
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if frac != "" {
		b.WriteByte(',')
		b.WriteString(frac)
	}
	return b.String()
}
