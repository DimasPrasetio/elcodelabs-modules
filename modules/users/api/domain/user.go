// Package domain holds the users module's entity and its rules.
package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"__GO_MODULE__/internal/shared/validator"
)

// User is a person who can sign in.
type User struct {
	ID        int64
	Name      string
	Email     string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
	// PasswordChangedAt is zero until the password is first changed.
	PasswordChangedAt time.Time
}

const (
	MaxNameLength     = 120
	MaxEmailLength    = 255
	MinPasswordLength = 8
	// MaxPasswordLength is bcrypt's limit: it ignores bytes past 72.
	MaxPasswordLength = 72
)

// NormalizeEmail is how every email is stored and looked up.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateProfile checks the name and email of a new or changed user.
func ValidateProfile(v *validator.Validator, name, email string) {
	v.Required("name", name)
	v.MaxLen("name", name, MaxNameLength)
	v.Required("email", email)
	v.MaxLen("email", email, MaxEmailLength)
	v.Email("email", email)
}

// ValidatePassword checks a new password.
func ValidatePassword(v *validator.Validator, field, password string) {
	v.Check(utf8.RuneCountInString(password) >= MinPasswordLength, field, "minimal 8 karakter")
	v.Check(len(password) <= MaxPasswordLength, field, "maksimal 72 byte")
}
