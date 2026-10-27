// Package taxid checks and normalizes Indonesian tax identities: the 16-digit NPWP, the NIK that
// serves as an individual's NPWP, the 22-digit NITKU of a place of business, and passport
// numbers for foreign parties. It checks formats only — whether a number is registered, or a
// party is a PKP, can be confirmed only by DJP (Coretax) and Dukcapil.
package taxid

import (
	"errors"
	"strings"
	"time"
)

// Kind is the type of a party's tax identity, as Coretax names a buyer's ID.
type Kind string

const (
	NPWP     Kind = "npwp"     // 16 digits: companies, government bodies, non-resident individuals
	NIK      Kind = "nik"      // 16 digits: resident individuals (the NIK is their NPWP, PMK 136/2023)
	Passport Kind = "passport" // foreign individuals
	None     Kind = "none"     // foreign companies, or no tax ID: name and address only
)

// Valid reports whether k is one of the kinds above.
func (k Kind) Valid() bool { return k == NPWP || k == NIK || k == Passport || k == None }

// ErrFormat is a number that does not have the shape its kind requires.
var ErrFormat = errors.New("taxid: wrong format")

// Digits keeps only the digits of s — "01.234.567.8-901.000" becomes "012345678901000".
func Digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// onlyDigitsAndPunctuation is s holding nothing but digits and the separators people write in
// an NPWP — so "12a3…" is refused rather than read as "123…".
func onlyDigitsAndPunctuation(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9') && !strings.ContainsRune(" .-/", r) {
			return false
		}
	}
	return true
}

// NormalizeNPWP returns the 16-digit NPWP of s: 16 digits as given, or a 15-digit NPWP of the
// old format prefixed with "0" (PMK 112/2022). Dots, dashes, and spaces are ignored. Anything
// else is ErrFormat.
func NormalizeNPWP(s string) (string, error) {
	if !onlyDigitsAndPunctuation(s) {
		return "", ErrFormat
	}
	d := Digits(s)
	switch len(d) {
	case 16:
		return d, nil
	case 15:
		return "0" + d, nil
	}
	return "", ErrFormat
}

// CheckNIK reports a NIK that is not 16 digits, has no region code, has a serial of 0000, or
// whose birth date (DDMMYY, day + 40 for women) is no real date. It cannot tell whether the NIK
// exists. Returns the NIK's digits.
func CheckNIK(s string) (string, error) {
	if !onlyDigitsAndPunctuation(s) {
		return "", ErrFormat
	}
	d := Digits(s)
	if len(d) != 16 || d[:2] == "00" || d[12:] == "0000" {
		return "", ErrFormat
	}
	day, month := num(d[6:8]), num(d[8:10])
	if day > 40 {
		day -= 40
	}
	// The year has two digits; 2000 is a leap year, so 29 February is a real date for "00".
	year := 2000 + num(d[10:12])
	if month < 1 || month > 12 || day < 1 || day > time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day() {
		return "", ErrFormat
	}
	return d, nil
}

func num(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

// CheckNITKU reports a NITKU that is not 22 digits, or — when npwp16 is given — does not start
// with that NPWP. The last six digits are the place's sequence; "000000" is the head office.
// Returns the NITKU's digits.
func CheckNITKU(nitku, npwp16 string) (string, error) {
	if !onlyDigitsAndPunctuation(nitku) {
		return "", ErrFormat
	}
	d := Digits(nitku)
	if len(d) != 22 {
		return "", ErrFormat
	}
	if npwp16 != "" && d[:16] != npwp16 {
		return "", ErrFormat
	}
	return d, nil
}

// CheckPassport reports a passport number that is not 5 to 20 letters and digits. Returns it in
// capitals.
func CheckPassport(s string) (string, error) {
	p := strings.ToUpper(strings.TrimSpace(s))
	if len(p) < 5 || len(p) > 20 {
		return "", ErrFormat
	}
	for _, r := range p {
		if !(r >= '0' && r <= '9') && !(r >= 'A' && r <= 'Z') {
			return "", ErrFormat
		}
	}
	return p, nil
}

// SuspectNPWP reports an NPWP of the old format (16 digits starting with "0") whose check digit
// does not match: a Luhn digit over its first nine digits, the way python-stdnum checks it. DJP
// has not published the algorithm, so this is a warning to show, never a reason to refuse. An
// NPWP that is a NIK has no check digit and is never suspect.
func SuspectNPWP(npwp16 string) bool {
	if len(npwp16) != 16 || npwp16[0] != '0' || Digits(npwp16) != npwp16 {
		return false
	}
	return !luhn(npwp16[1:10])
}

// luhn reports whether the last digit of d is the Luhn check digit of the ones before it.
func luhn(d string) bool {
	sum := 0
	for i := len(d) - 1; i >= 0; i-- {
		n := int(d[i] - '0')
		if (len(d)-1-i)%2 == 1 {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
	}
	return sum%10 == 0
}

// Mask hides all but the first and last four characters: "3201********0001". Shorter values
// keep only their last two.
func Mask(s string) string {
	r := []rune(s)
	switch {
	case len(r) == 0:
		return ""
	case len(r) <= 8:
		if len(r) <= 2 {
			return strings.Repeat("*", len(r))
		}
		return strings.Repeat("*", len(r)-2) + string(r[len(r)-2:])
	}
	return string(r[:4]) + strings.Repeat("*", len(r)-8) + string(r[len(r)-4:])
}
