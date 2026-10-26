package ports

import (
	"context"
	"errors"
)

// ErrWeakPassword means the new password breaks the store's rules; its text says how.
var ErrWeakPassword = errors.New("password does not meet the rules")

// PasswordWriter replaces a user's password once auth has proved who they are — for "change my
// password" and "reset by email". Optional: without it both are off. The store records when the
// password changed (User.PasswordChangedAt), which ends the user's earlier sessions.
type PasswordWriter interface {
	SetPassword(ctx context.Context, userID int64, password string) error
}

// Mailer sends one email. Optional: without it (or without a PasswordWriter), "forgot password"
// is off.
type Mailer interface {
	Send(ctx context.Context, to, toName, subject, text, html string) error
}
