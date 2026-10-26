// Package contracts is what the users module offers. Another module never imports it: it
// declares its own port, and an adapter in the composition root connects the two.
package contracts

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound means no user matches.
var ErrNotFound = errors.New("user not found")

// User is a user's public profile.
type User struct {
	ID       int64
	Name     string
	Email    string
	IsActive bool
	// PasswordChangedAt is zero until the password is first changed (GetByID only).
	PasswordChangedAt time.Time
}

// Credentials is what signing in checks against. Only an auth provider should ask for it.
type Credentials struct {
	UserID       int64
	PasswordHash string
	IsActive     bool
}

// UserClient is the users module's public contract.
type UserClient interface {
	// GetByID returns the user, or ErrNotFound.
	GetByID(ctx context.Context, id int64) (User, error)
	// GetByEmail returns the user with email (case-insensitive), or ErrNotFound.
	GetByEmail(ctx context.Context, email string) (User, error)
	// Search returns up to limit users whose name or email contains q, newest first.
	Search(ctx context.Context, q string, limit int) ([]User, error)
	// GetCredentialsByEmail returns the credentials for email (case-insensitive), or ErrNotFound.
	GetCredentialsByEmail(ctx context.Context, email string) (Credentials, error)
}
