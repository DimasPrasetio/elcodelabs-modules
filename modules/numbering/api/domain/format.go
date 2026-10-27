// Package domain is document numbering: formats, the series a number belongs to, and rendering.
package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"__GO_MODULE__/internal/kernel"
)

// Reset is when a series starts again from 1.
type Reset string

const (
	Never   Reset = "never"
	Yearly  Reset = "yearly"
	Monthly Reset = "monthly"
	Daily   Reset = "daily"
)

// Format is how a document type is numbered.
type Format struct {
	Pattern   string // "INV/{BRANCH}/{YYYY}/{MM}/{SEQ:4}"
	Reset     Reset
	PerBranch bool // each branch counts on its own
}

// DocType is what a document type may be called: "<module>.<thing>", lowercase.
var DocType = regexp.MustCompile(`^[a-z][a-z0-9_-]*(\.[a-z][a-z0-9_-]*)+$`)

// Limits: a pattern of at most MaxPattern characters must render numbers of at most MaxNumber
// characters with the longest codes (CodeLen) and counter (SeqDigits) — leaving room for an
// amendment suffix ("-12") in the consumer's VARCHAR(100).
const (
	MaxPattern = 60
	MaxNumber  = 90
	CodeLen    = 20 // organization's codes are VARCHAR(20); an ID has at most 19 digits
	SeqDigits  = 12 // the counter never passes MaxSeq
	MaxSeq     = 999_999_999_999
	MaxRaise   = 999_999_999 // a raised series keeps a thousand times its room
)

// Part is a piece of a parsed pattern: literal text, or a token.
type Part struct {
	Literal string
	Token   string // YYYY YY MM DD SEQ BRANCH COMPANY
	Width   int    // SEQ: zero-padded to at least this many digits
}

var tokenRe = regexp.MustCompile(`^\{(YYYY|YY|MM|DD|SEQ:(\d{1,2})|BRANCH|COMPANY)\}`)

// Parse splits a pattern into parts, or says in Indonesian what is wrong with it.
func Parse(pattern string) ([]Part, error) {
	if pattern == "" {
		return nil, fmt.Errorf("Pola wajib diisi")
	}
	if len(pattern) > MaxPattern {
		return nil, fmt.Errorf("Pola paling panjang %d karakter", MaxPattern)
	}
	var parts []Part
	for rest := pattern; rest != ""; {
		if rest[0] == '{' {
			m := tokenRe.FindStringSubmatch(rest)
			if m == nil {
				end := strings.IndexByte(rest, '}')
				if end < 0 {
					end = len(rest) - 1
				}
				return nil, fmt.Errorf("%q bukan penanda yang dikenal — pakai {YYYY} {YY} {MM} {DD} {SEQ:n} {BRANCH} {COMPANY}", rest[:end+1])
			}
			p := Part{Token: m[1]}
			if m[2] != "" {
				p.Token = "SEQ"
				p.Width, _ = strconv.Atoi(m[2])
				if p.Width < 1 || p.Width > 12 {
					return nil, fmt.Errorf("{SEQ:n}: n antara 1 dan 12")
				}
			}
			parts = append(parts, p)
			rest = rest[len(m[0]):]
			continue
		}
		end := strings.IndexByte(rest, '{')
		if end < 0 {
			end = len(rest)
		}
		lit := rest[:end]
		for _, r := range lit {
			if r == '}' || r > unicode.MaxASCII || !unicode.IsPrint(r) {
				return nil, fmt.Errorf("Teks pola hanya huruf, angka, dan tanda baca biasa (tanpa { })")
			}
		}
		parts = append(parts, Part{Literal: lit})
		rest = rest[end:]
	}
	return parts, nil
}

func has(parts []Part, tokens ...string) bool {
	for _, p := range parts {
		for _, t := range tokens {
			if p.Token == t {
				return true
			}
		}
	}
	return false
}

// variable tokens render with a width that changes (a code, or a counter past its padding).
func variable(p Part) bool { return p.Token == "BRANCH" || p.Token == "COMPANY" || p.Token == "SEQ" }

// separator reports whether a literal starts (first) or ends (!first) with a character that is
// neither a letter, a digit, nor "-" — codes may hold letters, digits, and "-".
func separator(lit string, first bool) bool {
	if lit == "" {
		return false
	}
	r := rune(lit[len(lit)-1])
	if first {
		r = rune(lit[0])
	}
	return r != '-' && !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

// amendable tells a number apart from a revision of another ("…-1", docstate): only a code
// can absorb that "-1" — the rest is digits of a fixed shape and literals the same for every
// number — so a code (which may end in "-1") never ends the pattern.
func amendable(parts []Part) bool {
	last := parts[len(parts)-1]
	return last.Token != "BRANCH" && last.Token != "COMPANY"
}

// Validate refuses a format that could give two documents the same number.
func (f Format) Validate() error {
	parts, err := Parse(f.Pattern)
	if err != nil {
		return err
	}
	seq := 0
	for _, p := range parts {
		if p.Token == "SEQ" {
			seq++
		}
	}
	if seq != 1 {
		return fmt.Errorf("Pola harus memuat tepat satu {SEQ:n}")
	}
	switch f.Reset {
	case Never:
	case Yearly:
		if !has(parts, "YYYY", "YY") {
			return fmt.Errorf("Mulai ulang tiap tahun butuh {YYYY} atau {YY} di pola, agar nomor tahun lalu tidak terulang")
		}
	case Monthly:
		if !has(parts, "YYYY", "YY") || !has(parts, "MM") {
			return fmt.Errorf("Mulai ulang tiap bulan butuh tahun ({YYYY} atau {YY}) dan {MM} di pola")
		}
	case Daily:
		if !has(parts, "YYYY", "YY") || !has(parts, "MM") || !has(parts, "DD") {
			return fmt.Errorf("Mulai ulang tiap hari butuh tahun, {MM}, dan {DD} di pola")
		}
	default:
		return fmt.Errorf("Mulai ulang: never, yearly, monthly, atau daily")
	}
	longest := 0
	for _, p := range parts {
		switch p.Token {
		case "":
			longest += len(p.Literal)
		case "YYYY":
			longest += 4
		case "YY", "MM", "DD":
			longest += 2
		case "SEQ":
			longest += max(p.Width, SeqDigits)
		case "BRANCH", "COMPANY":
			longest += CodeLen
		}
	}
	if longest > MaxNumber {
		return fmt.Errorf("Nomor dari pola ini bisa sampai %d karakter (paling panjang %d) — kurangi kode atau teks", longest, MaxNumber)
	}
	if f.PerBranch && !has(parts, "BRANCH") {
		return fmt.Errorf("Penomoran per cabang butuh {BRANCH} di pola, agar nomor cabang berbeda tidak sama")
	}
	// A code ({BRANCH}, {COMPANY}) is set apart from what is next to it by a separator — "/",
	// ".", "_", a space; not "-", which codes may hold — or the start or end; otherwise code "A1"
	// with 001 reads like code "A" with 1001.
	for i, p := range parts {
		if p.Token != "BRANCH" && p.Token != "COMPANY" {
			continue
		}
		if i > 0 && !(parts[i-1].Literal != "" && separator(parts[i-1].Literal, false)) {
			return fmt.Errorf("Beri pemisah (mis. / atau .) sebelum {%s} — bukan -, karena kode boleh memuat -", p.Token)
		}
		if i < len(parts)-1 && !(parts[i+1].Literal != "" && separator(parts[i+1].Literal, true)) {
			return fmt.Errorf("Beri pemisah (mis. / atau .) sesudah {%s} — bukan -, karena kode boleh memuat -", p.Token)
		}
	}
	for i := 1; i < len(parts); i++ {
		if variable(parts[i]) && variable(parts[i-1]) {
			return fmt.Errorf("Beri pemisah di antara {%s} dan {%s}", parts[i-1].Token, parts[i].Token)
		}
	}
	// A revision is the number with "-1", "-2" (docstate): no number may already look like one.
	if !amendable(parts) {
		return fmt.Errorf("Pola tidak boleh diakhiri {BRANCH} atau {COMPANY}: kode bisa berakhir -1, sehingga nomor lain sama dengan nomor revisi (…-1)")
	}
	return nil
}

// PeriodKey is the reset period date falls in: "" (never), "2026", "2026-10", "2026-10-27".
func PeriodKey(r Reset, date kernel.Date) string {
	switch r {
	case Yearly:
		return fmt.Sprintf("%04d", date.Year)
	case Monthly:
		return fmt.Sprintf("%04d-%02d", date.Year, date.Month)
	case Daily:
		return date.String()
	default:
		return ""
	}
}

// Codes are what {COMPANY} and {BRANCH} become.
type Codes struct {
	Company string
	Branch  string
}

// Render is the number of a valid format for date, counter value seq, and codes.
func (f Format) Render(date kernel.Date, seq int64, codes Codes) string {
	parts, _ := Parse(f.Pattern)
	var b strings.Builder
	for _, p := range parts {
		switch p.Token {
		case "":
			b.WriteString(p.Literal)
		case "YYYY":
			fmt.Fprintf(&b, "%04d", date.Year)
		case "YY":
			fmt.Fprintf(&b, "%02d", date.Year%100)
		case "MM":
			fmt.Fprintf(&b, "%02d", date.Month)
		case "DD":
			fmt.Fprintf(&b, "%02d", date.Day)
		case "SEQ":
			fmt.Fprintf(&b, "%0*d", p.Width, seq)
		case "BRANCH":
			b.WriteString(codes.Branch)
		case "COMPANY":
			b.WriteString(codes.Company)
		}
	}
	return b.String()
}
