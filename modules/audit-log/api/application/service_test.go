package application_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/audit-log/application"
	"__GO_MODULE__/internal/modules/audit-log/domain"
	"__GO_MODULE__/internal/shared/pagination"
)

type memRepo struct{ rows []domain.Entry }

func (m *memRepo) Insert(_ context.Context, e domain.Entry) error {
	e.ID = int64(len(m.rows) + 1)
	m.rows = append(m.rows, e)
	return nil
}

func (m *memRepo) List(_ context.Context, f application.Filter, _ pagination.Params) ([]domain.Entry, int64, error) {
	var out []domain.Entry
	for _, e := range m.rows {
		if (f.Module == "" || e.Module == f.Module) && !e.OccurredAt.Before(f.From) && e.OccurredAt.Before(f.To) {
			out = append(out, e)
		}
	}
	return out, int64(len(out)), nil
}

func (m *memRepo) Modules(context.Context) ([]string, error) { return nil, nil }

func (m *memRepo) DeleteOlderThan(_ context.Context, before time.Time) (int64, error) {
	kept := m.rows[:0]
	var n int64
	for _, e := range m.rows {
		if e.OccurredAt.Before(before) {
			n++
			continue
		}
		kept = append(kept, e)
	}
	m.rows = kept
	return n, nil
}

type directory map[int64]string

func (d directory) Names(_ context.Context, ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	for _, id := range ids {
		if n, ok := d[id]; ok {
			out[id] = n
		}
	}
	return out, nil
}

func TestWriteKeepsEveryRecordAndCutsOverlongValues(t *testing.T) {
	repo := &memRepo{}
	svc := application.NewService(repo, nil, 365*24*time.Hour)
	err := svc.Write(context.Background(), kernel.AuditRecord{
		AuditEntry: kernel.AuditEntry{Module: "users", Action: "user.updated", Target: "user:2", Summary: strings.Repeat("x", 1000), Data: map[string]any{"name": map[string]any{"before": "A", "after": "B"}}},
		At:         time.Now().UTC(), ActorID: 1, IP: "203.0.113.7", RequestID: "req-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	e := repo.rows[0]
	if len([]rune(e.Summary)) != domain.MaxSummary || !strings.Contains(string(e.Data), `"before":"A"`) || e.RequestID != "req-1" {
		t.Fatalf("entry = %+v", e)
	}
}

func TestListNamesActorsWhenItCan(t *testing.T) {
	repo := &memRepo{}
	now := time.Now().UTC()
	for _, actor := range []int64{1, 2, 0} {
		_ = repo.Insert(context.Background(), domain.Entry{OccurredAt: now, ActorID: actor, Module: "users", Action: "user.updated"})
	}
	named, _, err := application.NewService(repo, directory{1: "Ana"}, time.Hour).List(context.Background(), application.Filter{}, pagination.Params{Page: 1, Limit: 10})
	if err != nil || named[0].ActorName != "Ana" || named[1].ActorName != "" {
		t.Fatalf("named = %+v err=%v", named, err)
	}
	if _, _, err := application.NewService(repo, nil, time.Hour).List(context.Background(), application.Filter{}, pagination.Params{Page: 1, Limit: 10}); err != nil {
		t.Fatal("without a directory the trail must still list")
	}
}

func TestPruneDropsOnlyWhatIsPastRetention(t *testing.T) {
	repo := &memRepo{}
	now := time.Now().UTC()
	_ = repo.Insert(context.Background(), domain.Entry{OccurredAt: now.AddDate(0, 0, -400)})
	_ = repo.Insert(context.Background(), domain.Entry{OccurredAt: now.AddDate(0, 0, -10)})
	if err := application.NewService(repo, nil, 365*24*time.Hour).Prune(context.Background()); err != nil || len(repo.rows) != 1 {
		t.Fatalf("rows left %d, err %v", len(repo.rows), err)
	}
}
