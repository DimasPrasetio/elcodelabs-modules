// Package application holds the mailer's use cases: queueing a message, delivering what is due,
// and keeping the outbox tidy.
package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/mailer/contracts"
	"__GO_MODULE__/internal/modules/mailer/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/pagination"
)

// ErrNotFound means no message has that ID.
var ErrNotFound = errors.New("message not found")

// How a delivery run behaves. A claimed batch is leased for longer than the slowest it can
// take, so a message is only released to another run when its run is gone.
const (
	batchSize   = 10
	maxBatches  = 10 // per run: at most 100 messages, then the next run continues
	sendTimeout = time.Minute
	lease       = batchSize*sendTimeout + time.Minute
	markTimeout = 10 * time.Second
	keepSent    = 30 * 24 * time.Hour
	keepFailed  = 90 * 24 * time.Hour
)

// Repository is the outbox table.
type Repository interface {
	Enqueue(ctx context.Context, m domain.Message) (int64, error)
	// Claim marks up to limit due messages as sending until now+lease and returns them; messages
	// another run holds are skipped.
	Claim(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]domain.Message, error)
	// MarkSent records the delivery and clears the bodies: a sent message is never sent again,
	// and what it said (a reset link, say) does not stay in the database.
	MarkSent(ctx context.Context, id int64, at time.Time) error
	MarkFailedAttempt(ctx context.Context, id int64, status string, attempts int, next time.Time, lastErr string) error
	ReleaseStuck(ctx context.Context, now time.Time) (int64, error)
	Requeue(ctx context.Context, id int64, now time.Time) (bool, error)
	Prune(ctx context.Context, sentBefore, failedBefore time.Time) (int64, error)
	List(ctx context.Context, status, search string, p pagination.Params) ([]domain.Message, int64, error)
}

// Envelope is a message as a transport sends it.
type Envelope struct {
	ID       int64
	From     string
	FromName string
	To       string
	ToName   string
	Subject  string
	Text     string
	HTML     string // the complete document, layout included; empty for plain text
}

// Transport hands a message to a mail server (or, in development, to the log).
type Transport interface {
	Send(ctx context.Context, e Envelope) error
}

// PermanentError is a failure retrying will not fix — the mail server refused the recipient
// or the message for good (SMTP 5xx).
type PermanentError struct{ Err error }

func (e PermanentError) Error() string { return e.Err.Error() }
func (e PermanentError) Unwrap() error { return e.Err }

// Auditor records actions on the outbox — the kernel's d.Audit.
type Auditor interface {
	Record(ctx context.Context, e kernel.AuditEntry)
}

// Sender is who the app's email comes from.
type Sender struct {
	Address string
	Name    string
	AppName string // shown in the layout's header
}

// Service is the mailer's use cases.
type Service struct {
	repo      Repository
	transport Transport
	from      Sender
	audit     Auditor
	now       func() time.Time
}

func NewService(repo Repository, transport Transport, from Sender, audit Auditor) *Service {
	return &Service{repo: repo, transport: transport, from: from, audit: audit, now: func() time.Time { return time.Now().UTC() }}
}

// Send validates and queues a message; delivery happens in the background.
func (s *Service) Send(ctx context.Context, m contracts.Message) (int64, error) {
	if err := domain.Validate(m.To, m.ToName, m.Subject, m.Text, m.HTML); err != nil {
		return 0, fmt.Errorf("%w: %v", contracts.ErrInvalid, err)
	}
	now := s.now()
	return s.repo.Enqueue(ctx, domain.Message{To: m.To, ToName: m.ToName, Subject: m.Subject, Text: m.Text, HTML: m.HTML, NextAttemptAt: now, CreatedAt: now})
}

// DeliverDue sends what is due — the job the module schedules. Each message is attempted once
// per run; a failure is retried later with growing waits.
func (s *Service) DeliverDue(ctx context.Context) error {
	if _, err := s.repo.ReleaseStuck(ctx, s.now()); err != nil {
		return err
	}
	for range maxBatches {
		msgs, err := s.repo.Claim(ctx, s.now(), batchSize, lease)
		if err != nil {
			return err
		}
		for _, m := range msgs {
			if ctx.Err() != nil {
				return nil // shutting down: what is left is released when its lease ends
			}
			if err := s.deliver(ctx, m); err != nil {
				return err
			}
		}
		if len(msgs) < batchSize {
			return nil
		}
	}
	return nil
}

// deliver sends one message and records the outcome. A failed send is the message's problem
// (retried later); a failed record is the run's — the database is unreachable, so the run stops.
func (s *Service) deliver(ctx context.Context, m domain.Message) error {
	html := ""
	if m.HTML != "" {
		html = Layout(s.from.AppName, m.Subject, m.HTML)
	}
	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	err := s.transport.Send(sendCtx, Envelope{ID: m.ID, From: s.from.Address, FromName: s.from.Name, To: m.To, ToName: m.ToName, Subject: m.Subject, Text: m.Text, HTML: html})
	cancel()
	// The outcome is recorded even when the app is shutting down: an unrecorded delivery would be
	// sent again once its lease ends.
	markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), markTimeout)
	defer cancel()
	now := s.now()
	if err == nil {
		if err := s.repo.MarkSent(markCtx, m.ID, now); err != nil {
			return fmt.Errorf("mail %d was sent but not recorded, and may be sent again: %w", m.ID, err)
		}
		return nil
	}
	attempts := m.Attempts + 1
	status, next := domain.NextAttempt(attempts, errors.As(err, new(PermanentError)), now)
	msg := err.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	if err := s.repo.MarkFailedAttempt(markCtx, m.ID, status, attempts, next, msg); err != nil {
		return fmt.Errorf("mail %d: recording a failed attempt: %w", m.ID, err)
	}
	return nil
}

// Prune drops sent messages after 30 days and failed ones after 90.
func (s *Service) Prune(ctx context.Context) error {
	now := s.now()
	_, err := s.repo.Prune(ctx, now.Add(-keepSent), now.Add(-keepFailed))
	return err
}

// List is the outbox, newest first, optionally one status and a search on recipient/subject.
func (s *Service) List(ctx context.Context, status, search string, p pagination.Params) ([]domain.Message, int64, error) {
	switch status {
	case "", domain.StatusPending, domain.StatusSending, domain.StatusSent, domain.StatusFailed:
	default:
		return nil, 0, apperror.BadRequest("Status tidak dikenal: " + status)
	}
	return s.repo.List(ctx, status, search, p)
}

// Retry queues a failed message again, from its first attempt.
func (s *Service) Retry(ctx context.Context, id int64) error {
	ok, err := s.repo.Requeue(ctx, id, s.now())
	if err != nil {
		return err
	}
	if !ok {
		return apperror.Conflict("Hanya email yang gagal yang dapat dikirim ulang")
	}
	s.audit.Record(ctx, kernel.AuditEntry{Module: "mailer", Action: "mail.requeued", Target: fmt.Sprintf("mail:%d", id), Summary: "Mengirim ulang email yang gagal"})
	return nil
}
