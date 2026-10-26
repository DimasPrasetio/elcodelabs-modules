// Package domain holds the settings module's rules.
package domain

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"__GO_MODULE__/internal/shared/validator"
)

// Company is the "company" setting.
type Company struct {
	Name       string `json:"name"`
	LegalName  string `json:"legal_name"`
	Address    string `json:"address"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	TaxID      string `json:"tax_id"`
	Timezone   string `json:"timezone"`
	LogoFileID int64  `json:"logo_file_id"`
}

// DefaultTimezone is what a fresh install uses: Western Indonesia Time.
const DefaultTimezone = "Asia/Jakarta"

// Normalize trims the fields and fills the defaults.
func (c Company) Normalize() Company {
	for _, f := range []*string{&c.Name, &c.LegalName, &c.Address, &c.Phone, &c.Email, &c.TaxID, &c.Timezone} {
		*f = strings.TrimSpace(*f)
	}
	c.Email = strings.ToLower(c.Email)
	if c.Timezone == "" {
		c.Timezone = DefaultTimezone
	}
	return c
}

// Validate collects every field error.
func (c Company) Validate(v *validator.Validator) {
	v.Required("name", c.Name)
	v.MaxLen("name", c.Name, 120)
	v.MaxLen("legal_name", c.LegalName, 200)
	v.MaxLen("address", c.Address, 500)
	v.MaxLen("phone", c.Phone, 40)
	if c.Email != "" {
		a, err := mail.ParseAddress(c.Email)
		v.Check(err == nil && a.Address == c.Email, "email", "harus berupa alamat email yang valid")
	}
	if c.TaxID != "" {
		digits := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, c.TaxID)
		// NPWP: 15 digits (old format, often written 00.000.000.0-000.000) or 16 (NIK-based, since 2024).
		v.Check((len(digits) == 15 || len(digits) == 16) && utf8.RuneCountInString(c.TaxID) <= 25, "tax_id", "harus 15 atau 16 digit NPWP")
	}
	_, err := time.LoadLocation(c.Timezone)
	v.Check(err == nil && (strings.Contains(c.Timezone, "/") || c.Timezone == "UTC"), "timezone", "bukan zona waktu yang dikenal (contoh: Asia/Jakarta)")
}
