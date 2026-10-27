// Package domain is closing the books: lock dates per area, and the fiscal year.
package domain

import (
	"fmt"
	"regexp"
	"time"

	"__GO_MODULE__/internal/kernel"
)

// All is the area of the company-wide lock: it closes every area.
const All = "*"

// AreaName is what an area may be called: "sales", "payroll", "inventory.adjustments".
var AreaName = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,49}$`)

// Lock is how far one area of a company is closed.
type Lock struct {
	Area          string
	LockedThrough kernel.Date // this date and every date before it are closed
	UpdatedBy     int64
	UpdatedAt     time.Time
}

// Before reports whether a is an earlier day than b.
func Before(a, b kernel.Date) bool {
	if a.Year != b.Year {
		return a.Year < b.Year
	}
	if a.Month != b.Month {
		return a.Month < b.Month
	}
	return a.Day < b.Day
}

// Closing is the lock that closes area: the later of its own and the company-wide one. ok is
// false when neither exists.
func Closing(locks map[string]kernel.Date, area string) (through kernel.Date, ok bool) {
	all, hasAll := locks[All]
	own, hasOwn := locks[area]
	switch {
	case hasAll && hasOwn:
		if Before(own, all) {
			return all, true
		}
		return own, true
	case hasAll:
		return all, true
	default:
		return own, hasOwn
	}
}

// IsClosed reports whether date falls in a closed period of area.
func IsClosed(locks map[string]kernel.Date, area string, date kernel.Date) (kernel.Date, bool) {
	through, ok := Closing(locks, area)
	return through, ok && !Before(through, date)
}

// FiscalYear is the fiscal year date falls in, for a year starting in startMonth (1–12).
func FiscalYear(startMonth int, date kernel.Date) (start, end kernel.Date) {
	if startMonth < 1 || startMonth > 12 {
		startMonth = 1
	}
	y := date.Year
	if int(date.Month) < startMonth {
		y--
	}
	first := time.Date(y, time.Month(startMonth), 1, 12, 0, 0, 0, time.UTC)
	last := first.AddDate(1, 0, -1)
	return kernel.DateOf(first, time.UTC), kernel.DateOf(last, time.UTC)
}

// ParseDate reads "2026-09-30".
func ParseDate(s string) (kernel.Date, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return kernel.Date{}, fmt.Errorf("not a date: %q", s)
	}
	return kernel.DateOf(t, time.UTC), nil
}

// FormatDate is a date as Indonesians read it: "30 September 2026".
func FormatDate(d kernel.Date) string {
	return fmt.Sprintf("%d %s %d", d.Day, months[d.Month-1], d.Year)
}

var months = [...]string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
