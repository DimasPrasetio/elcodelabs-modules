// Package contracts is what the mailer module offers other modules (through their own ports
// and an adapter in the composition root).
package contracts

import (
	"context"
	"errors"
)

// ErrInvalid means the message cannot be sent as given (no recipient, a malformed address, a
// line break in a header, …). The error's text says which.
var ErrInvalid = errors.New("invalid message")

// Message is an email to one person.
type Message struct {
	To      string // address
	ToName  string // optional display name
	Subject string
	Text    string // the plain-text body — always required: some mail apps show only this
	// HTML is the inner body (paragraphs, a button); the mailer wraps it in the app's layout.
	// Optional: without it the email is plain text.
	HTML string
}

// MailClient queues email. Send returns once the message is stored — delivery happens in the
// background, with retries; a stored message is never lost to a slow or failing mail server.
type MailClient interface {
	Send(ctx context.Context, m Message) (id int64, err error)
}
