package kernel

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"testbed-app/internal/shared/request"
)

// AuditEntry is one change worth answering "who did what, when" for — d.Audit.Record. Never put
// secrets in Data (passwords, tokens); record what changed, not the whole row.
type AuditEntry struct {
	Module  string         // "users"
	Action  string         // "<thing>.<verb>", e.g. "user.deactivated"
	Target  string         // "<type>:<id>", e.g. "user:12"; empty when nothing specific
	Summary string         // one readable line, e.g. "Menonaktifkan Ana"
	Data    map[string]any // optional detail, e.g. {"before": …, "after": …}
	// Actor is who did it, when that is not the signed-in user (a sign-in, a reset by token).
	// Zero: the signed-in user, if any.
	Actor int64
}

// AuditRecord is an entry with what the kernel adds: when, by whom, from where.
type AuditRecord struct {
	AuditEntry
	At        time.Time
	ActorID   int64 // 0: no one signed in (system, or an anonymous request)
	IP        string
	RequestID string
}

// AuditSink stores records — what an audit module provides.
type AuditSink interface {
	Write(ctx context.Context, r AuditRecord) error
}

// AuditSlot is the audit trail every module receives in Deps.Audit. Like Auth and Authz it is a
// kernel slot: nearly every module that changes data records it. Without a provider a record is
// one log line; an audit module (the library's audit-log) stores it with Provide.
//
// Record runs after the change was committed, in the request that made it: a failure to store
// it is logged, never returned — the change itself already happened.
type AuditSlot struct {
	auth *AuthSlot
	log  *slog.Logger
	mu   sync.RWMutex
	sink AuditSink
}

// NewAuditSlot returns the slot; it learns who acted through auth.
func NewAuditSlot(auth *AuthSlot, log *slog.Logger) *AuditSlot {
	return &AuditSlot{auth: auth, log: log}
}

// Provide sets where records go. Called once, from internal/modules/registry.go.
func (s *AuditSlot) Provide(sink AuditSink) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sink = sink
}

// Record notes a change.
func (s *AuditSlot) Record(ctx context.Context, e AuditEntry) {
	r := AuditRecord{AuditEntry: e, At: time.Now().UTC(), ActorID: e.Actor, IP: request.ClientIPFrom(ctx), RequestID: request.RequestID(ctx)}
	if r.ActorID == 0 {
		r.ActorID, _ = s.auth.UserID(ctx)
	}
	s.mu.RLock()
	sink := s.sink
	s.mu.RUnlock()
	if sink == nil {
		s.log.Info("audit", "module", r.Module, "action", r.Action, "target", r.Target, "actor", r.ActorID, "summary", r.Summary)
		return
	}
	// The request may be ending; the record should still be written.
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err := sink.Write(wctx, r); err != nil {
		s.log.Error("audit: record not stored", "module", r.Module, "action", r.Action, "target", r.Target, "err", err)
	}
}
