// Package domain holds roles and the rules for granting permissions.
package domain

import (
	"strings"
	"time"

	"__GO_MODULE__/internal/shared/validator"
)

const (
	// AdminRole is created by the migration, holds Wildcard, and can be neither changed nor
	// deleted — only its members can.
	AdminRole = "Administrator"
	// Wildcard grants every permission, including ones modules declare later.
	Wildcard = "*"

	MaxNameLength        = 80
	MaxDescriptionLength = 255
)

// Role is a named set of permissions given to users.
type Role struct {
	ID          int64
	Name        string
	Description string
	IsSystem    bool
	Permissions []string
	MemberCount int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ValidateRole checks a role's name and description.
func ValidateRole(v *validator.Validator, name, description string) {
	v.Required("name", name)
	v.MaxLen("name", name, MaxNameLength)
	v.MaxLen("description", description, MaxDescriptionLength)
	v.Check(!strings.EqualFold(strings.TrimSpace(name), AdminRole), "name", "is reserved")
}

// Grants reports whether a set of granted permissions covers permission.
func Grants(granted map[string]bool, permission string) bool {
	return granted[Wildcard] || granted[permission]
}
