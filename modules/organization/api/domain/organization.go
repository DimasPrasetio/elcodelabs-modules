// Package domain holds companies, branches, placements, and their rules.
package domain

import (
	"regexp"
	"strings"
	"time"

	"__GO_MODULE__/internal/shared/validator"
)

// HomeCompany is company 1: the one existing data belongs to, never deactivated, and where
// companies are managed.
const HomeCompany int64 = 1

// Company is a legal entity.
type Company struct {
	ID        int64
	Code      string
	Name      string
	Timezone  string // IANA
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Branch is a place of business of a company — not a legal entity.
type Branch struct {
	ID        int64
	CompanyID int64
	Code      string
	Name      string
	Timezone  string // IANA; "" — the company's
	NITKU     string // 22 digits, or ""
	Address   string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Placement lets a user work in a company: across all of it (BranchID 0) or in one branch.
type Placement struct {
	UserID    int64
	CompanyID int64
	BranchID  int64
	IsDefault bool
	CreatedAt time.Time
	CreatedBy int64
}

// Limits of the stored fields.
const (
	MaxCodeLength    = 20
	MaxNameLength    = 120
	MaxAddressLength = 255
)

var (
	validCode  = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]*$`)
	validNITKU = regexp.MustCompile(`^[0-9]{22}$`)
)

// NormalizeCode is a code as stored: trimmed, upper case.
func NormalizeCode(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

func validateCode(v *validator.Validator, code string) {
	v.Required("code", code)
	v.MaxLen("code", code, MaxCodeLength)
	v.Check(code == "" || validCode.MatchString(code), "code", "hanya huruf besar, angka, dan tanda hubung")
}

// ValidTimezone reports whether tz names a time zone — IANA ("Asia/Makassar") or "UTC".
func ValidTimezone(tz string) bool {
	if tz != "UTC" && !strings.Contains(tz, "/") {
		return false
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}

// ValidateCompany checks a company's fields (code already normalized).
func ValidateCompany(v *validator.Validator, c Company) {
	validateCode(v, c.Code)
	v.Required("name", c.Name)
	v.MaxLen("name", c.Name, MaxNameLength)
	v.Check(ValidTimezone(c.Timezone), "timezone", "bukan zona waktu yang dikenal (contoh: Asia/Jakarta)")
}

// ValidateBranch checks a branch's fields (code already normalized).
func ValidateBranch(v *validator.Validator, b Branch) {
	validateCode(v, b.Code)
	v.Required("name", b.Name)
	v.MaxLen("name", b.Name, MaxNameLength)
	v.Check(b.Timezone == "" || ValidTimezone(b.Timezone), "timezone", "bukan zona waktu yang dikenal (contoh: Asia/Makassar)")
	v.Check(b.NITKU == "" || validNITKU.MatchString(b.NITKU), "nitku", "NITKU terdiri dari 22 angka (NPWP 16 digit + nomor urut 6 digit)")
	v.MaxLen("address", b.Address, MaxAddressLength)
}
