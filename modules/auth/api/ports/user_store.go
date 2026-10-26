// Package ports is what the auth module needs from outside, in its own words. Any provider
// satisfies it through an adapter in the composition root: the users module, or a project's
// own user table.
package ports

import (
	"context"
	"errors"
	"time"
)

// ErrUserNotFound means no user matches.
var ErrUserNotFound = errors.New("user not found")

// Credentials is what a sign-in is checked against.
type Credentials struct {
	UserID       int64
	PasswordHash string // bcrypt
	IsActive     bool
}

// User is who is signed in.
type User struct {
	ID       int64
	Name     string
	Email    string
	IsActive bool
	// PasswordChangedAt ends every session opened before it. A store that does not track it
	// leaves it zero, and sessions then end only on sign-out, deactivation, or expiry.
	PasswordChangedAt time.Time
}

// UserStore finds users for signing in and for the session.
type UserStore interface {
	// FindCredentials returns the credentials for email, or ErrUserNotFound.
	FindCredentials(ctx context.Context, email string) (Credentials, error)
	// FindUser returns the user, or ErrUserNotFound.
	FindUser(ctx context.Context, id int64) (User, error)
}
