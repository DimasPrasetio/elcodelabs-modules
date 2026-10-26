// Package application holds the mailer's use cases: queueing a message, delivering what is due,
// and keeping the outbox tidy.
package application

import (
	"context"
	"encoding/json"
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

// TaskDeliver is the task that sends one message (kernel d.Tasks); one per attempt.
const TaskDeliver = "mailer.deliver"

// How delivery behaves.
const (
	sendTimeout = time.Minute
	markTimeout = 10 * time.Second
	keepSent    = 30 * 24 * time.Hour
	keepFailed  = 90 * 24 * time.Hour
	// staleAfter is how long a message may sit due without a delivery before Rescue queues one.
	staleAfter = 10 * time.Minute
)

// Repository is the outbox table — the history of every email and its outcome. Delivery
// itself is a kernel task per attempt.
type Repository interface {
	Enqueue(ctx context.Context, m domain.Message) (int64, error)
	// Get returns the message, or ErrNotFound.
	Get(ctx context.Context, id int64) (domain.Message, error)
	// ClaimAttempt marks attempt as under way until until — only while it is the message's next
	// attempt and no other delivery of it holds the message; false otherwise.
	ClaimAttempt(ctx context.Context, id int64, attempt int, now, until time.Time) (bool, error)
	// ReleaseAttempt gives up the claim on attempt without counting it.
	ReleaseAttempt(ctx context.Context, id int64, attempt int) error
	// Postpone moves the due time of a message still waiting for attempt and due before
	// staleBefore — Rescue's claim on queuing that attempt; false when it is not.
	Postpone(ctx context.Context, id int64, attempt int, until, staleBefore time.Time) (bool, error)
	// MarkSent records the delivery and clears the bodies: a sent message is never sent again,
	// and what it said (a reset link, say) does not stay in the database.
	MarkSent(ctx context.Context, id int64, attempts int, at time.Time) error
	MarkFailedAttempt(ctx context.Context, id int64, status string, attempts int, next time.Time, lastErr string) error
	Requeue(ctx context.Context, id int64, now time.Time) (bool, error)
	Prune(ctx context.Context, sentBefore, failedBefore time.Time) (int64, error)
	// Stale lists up to 100 unsettled messages (pending or sending) due before before.
	Stale(ctx context.Context, before time.Time) ([]domain.Message, error)
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
	tx        *kernel.TxSlot
	tasks     *kernel.TasksSlot
	now       func() time.Time
}

func NewService(repo Repository, transport Transport, from Sender, audit Auditor, tx *kernel.TxSlot, tasks *kernel.TasksSlot) *Service {
	return &Service{repo: repo, transport: transport, from: from, audit: audit, tx: tx, tasks: tasks, now: func() time.Time { return time.Now().UTC() }}
}

// RegisterTasks declares the delivery task — called in New. It runs outside a transaction (it
// talks to a mail server) and records each outcome itself; a crash between the server's
// acceptance and that record sends the message again (at least once).
func (s *Service) RegisterTasks() {
	s.tasks.Register(TaskDeliver, s.Deliver, kernel.TaskKind{Lane: kernel.LaneShort, Timeout: 2 * sendTimeout, MaxAttempts: 10, NonTransactional: true})
}

// delivery is one attempt at one message.
type delivery struct {
	ID      int64 `json:"id"`
	Attempt int   `json:"attempt"`
}

func (s *Service) next(ctx context.Context, id int64, attempt int, at time.Time) error {
	return s.tasks.Enqueue(ctx, TaskDeliver, delivery{ID: id, Attempt: attempt}, kernel.At(at), kernel.Dedupe(fmt.Sprintf("%d:%d", id, attempt)))
}

// Send validates and queues a message: the outbox row and its first delivery commit together —
// inside the caller's d.Tx when there is one, so an email is sent only if what it tells about
// happened. The request never waits for a mail server.
func (s *Service) Send(ctx context.Context, m contracts.Message) (int64, error) {
	if err := domain.Validate(m.To, m.ToName, m.Subject, m.Text, m.HTML); err != nil {
		return 0, fmt.Errorf("%w: %v", contracts.ErrInvalid, err)
	}
	now := s.now()
	var id int64
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		if id, err = s.repo.Enqueue(ctx, domain.Message{To: m.To, ToName: m.ToName, Subject: m.Subject, Text: m.Text, HTML: m.HTML, NextAttemptAt: now, CreatedAt: now}); err != nil {
			return err
		}
		return s.next(ctx, id, 1, now)
	})
	return id, err
}

// Deliver makes one attempt at one message — the mailer.deliver task. A failure schedules the
// next attempt after 1 min, 5 min, 30 min, 2 h, then 6 h; a permanent refusal (SMTP 5xx) or the
// sixth failure marks the message failed for an administrator to retry.
func (s *Service) Deliver(ctx context.Context, t kernel.Task) error {
	var d delivery
	if err := json.Unmarshal(t.Payload, &d); err != nil {
		return fmt.Errorf("mailer: task %d: %w", t.ID, err)
	}
	m, err := s.repo.Get(ctx, d.ID)
	if errors.Is(err, ErrNotFound) {
		return nil // pruned
	}
	if err != nil {
		return err
	}
	if m.Status == domain.StatusSent || m.Status == domain.StatusFailed {
		return nil // already settled — an attempt repeated after a crash
	}
	// One delivery per attempt: another task for it (a rescue, a retry after a lost record) or a
	// later attempt already queued finds the message taken, and stops.
	if ok, err := s.repo.ClaimAttempt(ctx, m.ID, d.Attempt, s.now(), s.now().Add(2*sendTimeout+markTimeout)); err != nil || !ok {
		return err
	}
	html := ""
	if m.HTML != "" {
		html = Layout(s.from.AppName, m.Subject, m.HTML)
	}
	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	sendErr := s.transport.Send(sendCtx, Envelope{ID: m.ID, From: s.from.Address, FromName: s.from.Name, To: m.To, ToName: m.ToName, Subject: m.Subject, Text: m.Text, HTML: html})
	cancel()
	// The outcome is recorded even when the app is shutting down: an unrecorded delivery would
	// be sent again.
	markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), markTimeout)
	defer cancel()
	now := s.now()
	if sendErr != nil && ctx.Err() != nil {
		// Cut short by shutdown: not the message's failure. The task goes back in the queue and
		// this attempt is made again.
		if err := s.repo.ReleaseAttempt(markCtx, m.ID, d.Attempt); err != nil {
			return errors.Join(ctx.Err(), err)
		}
		return ctx.Err()
	}
	if sendErr == nil {
		if err := s.repo.MarkSent(markCtx, m.ID, d.Attempt, now); err != nil {
			return fmt.Errorf("mail %d was sent but not recorded, and may be sent again: %w", m.ID, err)
		}
		return nil
	}
	status, next := domain.NextAttempt(d.Attempt, errors.As(sendErr, new(PermanentError)), now)
	msg := sendErr.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	return s.tx.Do(markCtx, func(ctx context.Context) error {
		if err := s.repo.MarkFailedAttempt(ctx, m.ID, status, d.Attempt, next, msg); err != nil {
			return err
		}
		if status == domain.StatusFailed {
			return nil
		}
		return s.next(ctx, m.ID, d.Attempt+1, next)
	})
}

// Rescue queues the next attempt of every message long due with no delivery under way — left
// by an upgrade from the mailer's own worker (0.1), or by a delivery task that failed for good
// before it could record the outcome. A message whose delivery task still waits is untouched
// (the task's dedupe key, "<id>:<attempt>", is the same); one being sent was marked when the
// attempt started. Each rescued message is postponed, so the next run reaches the ones after it.
func (s *Service) Rescue(ctx context.Context) error {
	stale, err := s.repo.Stale(ctx, s.now().Add(-staleAfter))
	if err != nil {
		return err
	}
	for _, m := range stale {
		err := s.tx.Do(ctx, func(ctx context.Context) error {
			ok, err := s.repo.Postpone(ctx, m.ID, m.Attempts+1, s.now(), s.now().Add(-staleAfter))
			if err != nil || !ok {
				return err // settled, attempted, or rescued since it was listed
			}
			return s.next(ctx, m.ID, m.Attempts+1, s.now())
		})
		if err != nil {
			return err
		}
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
	now := s.now()
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		ok, err := s.repo.Requeue(ctx, id, now)
		if err != nil {
			return err
		}
		if !ok {
			return apperror.Conflict("Hanya email yang gagal yang dapat dikirim ulang")
		}
		return s.next(ctx, id, 1, now)
	})
	if err != nil {
		return err
	}
	s.audit.Record(ctx, kernel.AuditEntry{Module: "mailer", Action: "mail.requeued", Target: fmt.Sprintf("mail:%d", id), Summary: "Mengirim ulang email yang gagal"})
	return nil
}
