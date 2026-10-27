// Package ports is what the organization module needs from outside: the people it places.
// Any provider satisfies it through an adapter in the composition root — the users module, or a
// project's own user table.
package ports

import (
	"context"
	"errors"
)

// ErrUserNotFound means no user matches.
var ErrUserNotFound = errors.New("user not found")

// User is someone who can be placed.
type User struct {
	ID       int64
	Name     string
	Email    string
	IsActive bool
}

// UserDirectory finds users.
type UserDirectory interface {
	// Get returns the user, or ErrUserNotFound.
	Get(ctx context.Context, id int64) (User, error)
	// FindByEmail returns the user with email, or ErrUserNotFound.
	FindByEmail(ctx context.Context, email string) (User, error)
	// Search returns up to limit users whose name or email contains q.
	Search(ctx context.Context, q string, limit int) ([]User, error)
	// SearchAmong is Search among ids only.
	SearchAmong(ctx context.Context, ids []int64, q string, limit int) ([]User, error)
	// IDs returns every user's ID — to place everyone when the module is installed.
	IDs(ctx context.Context) ([]int64, error)
}
