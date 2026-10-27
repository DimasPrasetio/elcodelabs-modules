package domain

import (
	"testing"

	"__GO_MODULE__/internal/kernel"
)

func d(s string) kernel.Date {
	x, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return x
}

func TestIsClosed(t *testing.T) {
	locks := map[string]kernel.Date{All: d("2025-09-30"), "sales": d("2025-10-31"), "payroll": d("2025-08-31")}
	cases := []struct {
		area, date string
		closed     bool
		through    string
	}{
		{"sales", "2025-10-31", true, "2025-10-31"},
		{"sales", "2025-11-01", false, ""},
		{"payroll", "2025-09-15", true, "2025-09-30"}, // the company-wide lock is later
		{"payroll", "2025-10-01", false, ""},
		{"inventory", "2025-09-30", true, "2025-09-30"},
		{"inventory", "2025-10-01", false, ""},
	}
	for _, c := range cases {
		through, closed := IsClosed(locks, c.area, d(c.date))
		if closed != c.closed || (closed && through != d(c.through)) {
			t.Errorf("%s on %s: %v %v", c.area, c.date, through, closed)
		}
	}
	if _, closed := IsClosed(map[string]kernel.Date{}, "sales", d("1990-01-01")); closed {
		t.Error("no locks")
	}
}

func TestFiscalYearEdges(t *testing.T) {
	s, e := FiscalYear(4, d("2028-02-29"))
	if s != d("2027-04-01") || e != d("2028-03-31") {
		t.Errorf("April year, leap day: %v – %v", s, e)
	}
	s, e = FiscalYear(0, d("2026-05-05"))
	if s != d("2026-01-01") || e != d("2026-12-31") {
		t.Errorf("bad month falls to January: %v – %v", s, e)
	}
}

func TestBeforeAndFormat(t *testing.T) {
	if !Before(d("2025-12-31"), d("2026-01-01")) || Before(d("2026-01-01"), d("2026-01-01")) || !Before(d("2026-01-31"), d("2026-02-01")) {
		t.Error("Before")
	}
	if FormatDate(d("2026-07-01")) != "1 Juli 2026" {
		t.Error(FormatDate(d("2026-07-01")))
	}
	if _, err := ParseDate("2026-02-30"); err == nil {
		t.Error("ParseDate accepted 30 February")
	}
}
