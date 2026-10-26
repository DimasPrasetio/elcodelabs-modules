// Package domain holds the mailer's rules: what a valid message is, and how delivery retries.
package domain

import (
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

// Status of a queued message.
const (
	StatusPending = "pending" // waiting for its next attempt
	StatusSending = "sending" // claimed by a delivery run
	StatusSent    = "sent"
	StatusFailed  = "failed" // gave up: a permanent error, or every retry used
)

// Limits of a message.
const (
	MaxSubject = 200
	MaxName    = 120
	MaxBody    = 1 << 20
)

// Message is a queued email.
type Message struct {
	ID            int64
	To            string
	ToName        string
	Subject       string
	Text          string
	HTML          string // inner body; empty for plain text
	Status        string
	Attempts      int
	NextAttemptAt time.Time
	LastError     string
	CreatedAt     time.Time
	SentAt        time.Time
}

// Validate reports what is wrong with a message, or nil. Header fields may not contain line
// breaks: that is how a header injection would add recipients.
func Validate(to, toName, subject, text, html string) error {
	switch {
	case strings.TrimSpace(to) == "":
		return fmt.Errorf("recipient is empty")
	case strings.ContainsAny(to+toName+subject, "\r\n"):
		return fmt.Errorf("a header contains a line break")
	case strings.TrimSpace(subject) == "":
		return fmt.Errorf("subject is empty")
	case utf8.RuneCountInString(subject) > MaxSubject:
		return fmt.Errorf("subject is longer than %d characters", MaxSubject)
	case utf8.RuneCountInString(toName) > MaxName:
		return fmt.Errorf("recipient name is longer than %d characters", MaxName)
	case strings.TrimSpace(text) == "":
		return fmt.Errorf("the plain-text body is empty")
	case len(text) > MaxBody || len(html) > MaxBody:
		return fmt.Errorf("the body is larger than %d bytes", MaxBody)
	}
	a, err := mail.ParseAddress(to)
	if err != nil || a.Address != strings.TrimSpace(to) {
		return fmt.Errorf("recipient %q is not an email address", to)
	}
	return nil
}

// retryAfter is how long to wait before attempt n+1, after n failed attempts.
var retryAfter = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour}

// MaxAttempts is how many times delivery is tried before the message is marked failed.
var MaxAttempts = len(retryAfter) + 1

// NextAttempt says what happens after a failed attempt: the attempt count so far (including
// this one) decides the wait; a permanent error, or the last attempt, gives up.
func NextAttempt(attempts int, permanent bool, now time.Time) (status string, next time.Time) {
	if permanent || attempts >= MaxAttempts {
		return StatusFailed, now
	}
	return StatusPending, now.Add(retryAfter[attempts-1])
}
