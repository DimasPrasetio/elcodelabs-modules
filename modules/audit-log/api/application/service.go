// Package application holds the audit trail's use cases: storing what the kernel hands over,
// and reading it back.
package application

import (
	"context"
	"encoding/json"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/audit-log/domain"
	"__GO_MODULE__/internal/modules/audit-log/ports"
	"__GO_MODULE__/internal/shared/pagination"
)

// Filter narrows the trail. From and To are a half-open interval.
type Filter struct {
	Company int64 // set by List from the request's scope
	Kind    string
	Module  string
	Actor   int64
	Target  string
	From    time.Time
	To      time.Time
}

// Repository is the audit_log table.
type Repository interface {
	Insert(ctx context.Context, e domain.Entry) error
	List(ctx context.Context, f Filter, p pagination.Params) ([]domain.Entry, int64, error)
	Modules(ctx context.Context, company int64) ([]string, error)
	SystemDeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// Service is the audit trail.
type Service struct {
	repo      Repository
	users     ports.UserDirectory // nil: actors shown by ID
	retention time.Duration
	now       func() time.Time
}

func NewService(repo Repository, users ports.UserDirectory, retention time.Duration) *Service {
	return &Service{repo: repo, users: users, retention: retention, now: func() time.Time { return time.Now().UTC() }}
}

// Write stores a record — the kernel's AuditSink. Over-long values are cut, not refused.
func (s *Service) Write(ctx context.Context, r kernel.AuditRecord) error {
	data := json.RawMessage("null")
	if len(r.Data) > 0 {
		b, err := json.Marshal(r.Data)
		if err != nil || len(b) > domain.MaxData {
			b, _ = json.Marshal(map[string]any{"note": "data too large or not JSON"})
		}
		data = b
	}
	kind := string(r.Kind)
	if kind == "" {
		kind = string(kernel.AuditChange)
	}
	company := r.CompanyID
	if company == 0 {
		company = kernel.DefaultCompany
	}
	return s.repo.Insert(ctx, domain.Entry{
		OccurredAt: r.At, ActorID: r.ActorID, Kind: domain.Cut(kind, 10), CompanyID: company, BranchID: r.BranchID,
		Module: domain.Cut(r.Module, domain.MaxModule), Action: domain.Cut(r.Action, domain.MaxAction),
		Target: domain.Cut(r.Target, domain.MaxTarget), Summary: domain.Cut(r.Summary, domain.MaxSummary),
		Data: data, IP: domain.Cut(r.IP, 45), RequestID: domain.Cut(r.RequestID, 64),
	})
}

// Named is an entry with its actor's display name.
type Named struct {
	domain.Entry
	ActorName string
}

// List is the active company's trail, newest first, with each actor's name when the directory
// knows it.
func (s *Service) List(ctx context.Context, f Filter, p pagination.Params) ([]Named, int64, error) {
	f.Company = kernel.ScopeFrom(ctx).CompanyID
	// An open end is a real bound: MySQL compares a TIMESTAMP with Go's zero time ("0000-00-00")
	// as no match at all.
	if f.From.IsZero() {
		f.From = time.Date(1970, 1, 2, 0, 0, 0, 0, time.UTC)
	}
	if f.To.IsZero() {
		f.To = s.now().Add(time.Minute)
	}
	entries, total, err := s.repo.List(ctx, f, p)
	if err != nil {
		return nil, 0, err
	}
	names := map[int64]string{}
	if s.users != nil {
		var ids []int64
		seen := map[int64]bool{}
		for _, e := range entries {
			if e.ActorID != 0 && !seen[e.ActorID] {
				seen[e.ActorID] = true
				ids = append(ids, e.ActorID)
			}
		}
		if len(ids) > 0 {
			if got, err := s.users.Names(ctx, ids); err == nil {
				names = got
			}
		}
	}
	out := make([]Named, len(entries))
	for i, e := range entries {
		out[i] = Named{Entry: e, ActorName: names[e.ActorID]}
	}
	return out, total, nil
}

func (s *Service) Modules(ctx context.Context) ([]string, error) {
	return s.repo.Modules(ctx, kernel.ScopeFrom(ctx).CompanyID)
}

// Prune removes entries older than the retention period — the job the module schedules.
func (s *Service) Prune(ctx context.Context) error {
	before := s.now().Add(-s.retention)
	for range 20 { // at most 100 000 rows a run, in short statements
		n, err := s.repo.SystemDeleteOlderThan(ctx, before)
		if err != nil || n < 5000 || ctx.Err() != nil {
			return err
		}
	}
	return nil
}
