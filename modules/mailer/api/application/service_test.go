package application_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/mailer/application"
	"__GO_MODULE__/internal/modules/mailer/contracts"
	"__GO_MODULE__/internal/modules/mailer/domain"
	"__GO_MODULE__/internal/shared/pagination"
)

// memRepo is the outbox in memory: the module is tested without MySQL.
type memRepo struct {
	mu   sync.Mutex
	rows map[int64]*domain.Message
	next int64
}

func newRepo() *memRepo { return &memRepo{rows: map[int64]*domain.Message{}} }

func (r *memRepo) Enqueue(_ context.Context, m domain.Message) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	m.ID, m.Status = r.next, domain.StatusPending
	r.rows[m.ID] = &m
	return m.ID, nil
}

func (r *memRepo) Claim(_ context.Context, now time.Time, limit int, _ time.Duration) ([]domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ids []int64
	for id, m := range r.rows {
		if m.Status == domain.StatusPending && !m.NextAttemptAt.After(now) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var out []domain.Message
	for _, id := range ids {
		if len(out) == limit {
			break
		}
		r.rows[id].Status = domain.StatusSending
		out = append(out, *r.rows[id])
	}
	return out, nil
}

func (r *memRepo) MarkSent(_ context.Context, id int64, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[id].Status, r.rows[id].SentAt = domain.StatusSent, at
	r.rows[id].Attempts++
	return nil
}

func (r *memRepo) MarkFailedAttempt(_ context.Context, id int64, status string, attempts int, next time.Time, lastErr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.rows[id]
	m.Status, m.Attempts, m.NextAttemptAt, m.LastError = status, attempts, next, lastErr
	return nil
}

func (r *memRepo) ReleaseStuck(context.Context, time.Time) (int64, error) { return 0, nil }

func (r *memRepo) Requeue(_ context.Context, id int64, now time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.rows[id]
	if !ok || m.Status != domain.StatusFailed {
		return false, nil
	}
	m.Status, m.Attempts, m.NextAttemptAt = domain.StatusPending, 0, now
	return true, nil
}

func (r *memRepo) Prune(context.Context, time.Time, time.Time) (int64, error) { return 0, nil }

func (r *memRepo) List(context.Context, string, string, pagination.Params) ([]domain.Message, int64, error) {
	return nil, 0, nil
}

// transport records what it was asked to send and fails as told.
type transport struct {
	sent []application.Envelope
	fail error
}

func (t *transport) Send(_ context.Context, e application.Envelope) error {
	if t.fail != nil {
		return t.fail
	}
	t.sent = append(t.sent, e)
	return nil
}

type noAudit struct{}

func (noAudit) Record(context.Context, kernel.AuditEntry) {}

func setup() (*application.Service, *memRepo, *transport) {
	repo, tr := newRepo(), &transport{}
	return application.NewService(repo, tr, application.Sender{Address: "noreply@app.test", Name: "App", AppName: "App"}, noAudit{}), repo, tr
}

func TestSendQueuesAndTheJobDelivers(t *testing.T) {
	svc, repo, tr := setup()
	id, err := svc.Send(context.Background(), contracts.Message{To: "ana@example.com", Subject: "Halo", Text: "Isi", HTML: "<p>Isi</p>"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.sent) != 0 {
		t.Fatal("Send must not wait for the mail server")
	}
	if err := svc.DeliverDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(tr.sent) != 1 || repo.rows[id].Status != domain.StatusSent {
		t.Fatalf("sent=%d status=%s", len(tr.sent), repo.rows[id].Status)
	}
	if !strings.Contains(tr.sent[0].HTML, "<p>Isi</p>") || !strings.Contains(tr.sent[0].HTML, "<html") {
		t.Fatal("the inner HTML must be wrapped in the layout")
	}
}

func TestInvalidMessagesAreRefused(t *testing.T) {
	svc, _, _ := setup()
	for name, m := range map[string]contracts.Message{
		"no recipient":     {Subject: "s", Text: "t"},
		"bad address":      {To: "not-an-email", Subject: "s", Text: "t"},
		"header injection": {To: "a@example.com", Subject: "s\r\nBcc: x@evil.example", Text: "t"},
		"no text":          {To: "a@example.com", Subject: "s"},
	} {
		if _, err := svc.Send(context.Background(), m); !errors.Is(err, contracts.ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
}

func TestTemporaryFailuresRetryThenGiveUp(t *testing.T) {
	svc, repo, tr := setup()
	id, _ := svc.Send(context.Background(), contracts.Message{To: "ana@example.com", Subject: "s", Text: "t"})
	tr.fail = errors.New("421 try later")
	for i := 1; i < domain.MaxAttempts; i++ {
		repo.rows[id].NextAttemptAt = time.Time{} // due now
		_ = svc.DeliverDue(context.Background())
		if repo.rows[id].Status != domain.StatusPending || repo.rows[id].Attempts != i {
			t.Fatalf("attempt %d: status=%s attempts=%d", i, repo.rows[id].Status, repo.rows[id].Attempts)
		}
	}
	repo.rows[id].NextAttemptAt = time.Time{}
	_ = svc.DeliverDue(context.Background())
	if repo.rows[id].Status != domain.StatusFailed {
		t.Fatalf("after %d attempts: status=%s, want failed", domain.MaxAttempts, repo.rows[id].Status)
	}
	if err := svc.Retry(context.Background(), id); err != nil || repo.rows[id].Status != domain.StatusPending {
		t.Fatalf("retry: %v %s", err, repo.rows[id].Status)
	}
}

func TestPermanentFailureGivesUpAtOnce(t *testing.T) {
	svc, repo, tr := setup()
	id, _ := svc.Send(context.Background(), contracts.Message{To: "nobody@example.com", Subject: "s", Text: "t"})
	tr.fail = application.PermanentError{Err: errors.New("550 no such user")}
	_ = svc.DeliverDue(context.Background())
	if repo.rows[id].Status != domain.StatusFailed || !strings.Contains(repo.rows[id].LastError, "550") {
		t.Fatalf("status=%s error=%q", repo.rows[id].Status, repo.rows[id].LastError)
	}
}
