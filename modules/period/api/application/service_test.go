package application_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/kernel/kerneltest"
	"__GO_MODULE__/internal/modules/period/application"
	"__GO_MODULE__/internal/modules/period/contracts"
	"__GO_MODULE__/internal/modules/period/domain"
	"__GO_MODULE__/internal/shared/apperror"
)

// memRepo is the two tables in memory; it records which hold each call took.
type memRepo struct {
	locks  map[int64]map[string]domain.Lock
	fiscal map[int64]int
	holds  []string
}

func newMemRepo() *memRepo {
	return &memRepo{locks: map[int64]map[string]domain.Lock{}, fiscal: map[int64]int{}}
}

func (m *memRepo) HoldShared(_ context.Context, c int64) error {
	m.holds = append(m.holds, "shared")
	return nil
}

func (m *memRepo) HoldExclusive(_ context.Context, c int64) error {
	m.holds = append(m.holds, "exclusive")
	return nil
}

func (m *memRepo) LocksOfArea(_ context.Context, c int64, area string, shared bool) (map[string]kernel.Date, error) {
	out := map[string]kernel.Date{}
	for _, a := range []string{domain.All, area} {
		if l, ok := m.locks[c][a]; ok {
			out[a] = l.LockedThrough
		}
	}
	return out, nil
}

func (m *memRepo) Locks(_ context.Context, c int64) ([]domain.Lock, error) {
	var out []domain.Lock
	for _, l := range m.locks[c] {
		out = append(out, l)
	}
	return out, nil
}

func (m *memRepo) PutLock(_ context.Context, c int64, l domain.Lock) error {
	if m.locks[c] == nil {
		m.locks[c] = map[string]domain.Lock{}
	}
	m.locks[c][l.Area] = l
	return nil
}

func (m *memRepo) DeleteLock(_ context.Context, c int64, area string) error {
	delete(m.locks[c], area)
	return nil
}

func (m *memRepo) FiscalStart(_ context.Context, c int64) (int, error) {
	if v, ok := m.fiscal[c]; ok {
		return v, nil
	}
	return 1, nil
}

func (m *memRepo) SetFiscalStart(_ context.Context, c int64, month int, _ time.Time) error {
	m.fiscal[c] = month
	return nil
}

// authz grants permissions across the company, or in one branch only.
type authz map[string]kernel.Units

func (a authz) Units(_ context.Context, p string) (kernel.Units, error) { return a[p], nil }

type signedIn struct{}

func (signedIn) UserID(context.Context) (int64, bool) { return 7, true }

var all = kernel.Units{AllBranches: true}

func setup(t *testing.T, perms authz) (*application.Service, *memRepo, *kerneltest.Kit) {
	t.Helper()
	k := kerneltest.Deps(t) // today is 2026-01-02 in WIB
	repo := newMemRepo()
	svc := application.NewService(repo, k.Deps.Tx, k.Deps.Audit, perms, signedIn{}, k.Clock)
	svc.Declare("sales", "Penjualan")
	svc.Declare("payroll", "Penggajian")
	return svc, repo, k
}

func date(s string) *kernel.Date {
	d, err := domain.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return &d
}

func status(t *testing.T, err error) int {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok {
		t.Fatalf("want *apperror.Error, got %v", err)
	}
	return e.Status
}

func in(company int64) context.Context { return kerneltest.In(context.Background(), company, 0) }

func TestEnsureOpen(t *testing.T) {
	svc, repo, _ := setup(t, authz{application.PermManage: all, application.PermUnlock: all})
	ctx := in(1)
	if err := svc.EnsureOpen(ctx, "sales", *date("2025-01-01")); err != nil {
		t.Fatalf("no lock: %v", err)
	}
	if _, err := svc.SetLock(ctx, "sales", date("2025-10-31"), ""); err != nil {
		t.Fatal(err)
	}
	err := svc.EnsureOpen(ctx, "sales", *date("2025-10-31"))
	var closed *contracts.ClosedError
	if !errors.Is(err, contracts.ErrClosed) || !errors.As(err, &closed) || closed.Through != *date("2025-10-31") {
		t.Fatalf("on the lock date: %v", err)
	}
	if err.Error() != "Buku Penjualan sudah ditutup sampai 31 Oktober 2025 — tanggal 31 Oktober 2025 ada di periode yang ditutup" {
		t.Errorf("message: %q", err)
	}
	if err := svc.EnsureOpen(ctx, "sales", *date("2025-11-01")); err != nil {
		t.Errorf("the day after: %v", err)
	}
	if err := svc.EnsureOpen(ctx, "payroll", *date("2025-10-01")); err != nil {
		t.Errorf("another area: %v", err)
	}
	// The company-wide lock closes every area, and wins when later.
	if _, err := svc.SetLock(ctx, domain.All, date("2025-11-30"), ""); err != nil {
		t.Fatal(err)
	}
	for _, a := range []string{"sales", "payroll", domain.All} {
		if err := svc.EnsureOpen(ctx, a, *date("2025-11-15")); !errors.Is(err, contracts.ErrClosed) {
			t.Errorf("%s under the company-wide lock: %v", a, err)
		}
	}
	// Another company is untouched.
	if err := svc.EnsureOpen(in(2), "sales", *date("2025-01-01")); err != nil {
		t.Errorf("company 2: %v", err)
	}
	if err := svc.EnsureOpen(ctx, "nope", *date("2025-01-01")); !errors.Is(err, application.ErrUnknownArea) {
		t.Errorf("undeclared area: %v", err)
	}

	// Outside a transaction it only reads; inside it holds the company's row shared.
	repo.holds = nil
	_ = svc.EnsureOpen(ctx, "sales", *date("2026-01-01"))
	k := kerneltest.Deps(t)
	_ = k.Deps.Tx.Do(ctx, func(ctx context.Context) error { return svc.EnsureOpen(ctx, "sales", *date("2026-01-01")) })
	if len(repo.holds) != 1 || repo.holds[0] != "shared" {
		t.Errorf("holds = %v; want one shared hold, inside the transaction", repo.holds)
	}
}

func TestClosingNeedsManageAcrossTheCompany(t *testing.T) {
	svc, _, _ := setup(t, authz{application.PermManage: {BranchIDs: []int64{3}}})
	if _, err := svc.SetLock(in(1), "sales", date("2025-10-31"), ""); status(t, err) != http.StatusForbidden {
		t.Fatalf("manage in one branch only: %v", err)
	}
	svc, repo, k := setup(t, authz{application.PermManage: all})
	l, err := svc.SetLock(in(1), "sales", date("2025-10-31"), "")
	if err != nil || l.LockedThrough != *date("2025-10-31") || l.UpdatedBy != 7 {
		t.Fatalf("SetLock = %+v, %v", l, err)
	}
	if repo.holds[0] != "exclusive" {
		t.Errorf("a lock change holds the company exclusive: %v", repo.holds)
	}
	recs := k.Audit.Records()
	if len(recs) != 1 || recs[0].Action != "lock.closed" || recs[0].Kind == kernel.AuditSecurity {
		t.Errorf("audit = %+v", recs)
	}
	// Moving it forward again is still closing.
	if _, err := svc.SetLock(in(1), "sales", date("2025-11-30"), ""); err != nil {
		t.Errorf("forward: %v", err)
	}
	// The same date again changes nothing and records nothing.
	if _, err := svc.SetLock(in(1), "sales", date("2025-11-30"), ""); err != nil || len(k.Audit.Records()) != 2 {
		t.Errorf("no-op: %v, %d records", err, len(k.Audit.Records()))
	}
}

func TestReopeningNeedsUnlockAndAReason(t *testing.T) {
	svc, repo, k := setup(t, authz{application.PermManage: all})
	if _, err := svc.SetLock(in(1), "sales", date("2025-10-31"), ""); err != nil {
		t.Fatal(err)
	}
	for _, through := range []*kernel.Date{date("2025-09-30"), nil} {
		if _, err := svc.SetLock(in(1), "sales", through, "salah tutup"); status(t, err) != http.StatusForbidden {
			t.Fatalf("reopen to %v without period.unlock: %v", through, err)
		}
	}
	if repo.locks[1]["sales"].LockedThrough != *date("2025-10-31") {
		t.Fatal("a refused reopen changed the lock")
	}

	svc, repo, k = setup(t, authz{application.PermManage: all, application.PermUnlock: {BranchIDs: []int64{3}}})
	_, _ = svc.SetLock(in(1), "sales", date("2025-10-31"), "")
	if _, err := svc.SetLock(in(1), "sales", date("2025-09-30"), "salah tutup"); status(t, err) != http.StatusForbidden {
		t.Fatalf("unlock in one branch only: %v", err)
	}

	svc, repo, k = setup(t, authz{application.PermManage: all, application.PermUnlock: all})
	_, _ = svc.SetLock(in(1), "sales", date("2025-10-31"), "")
	if _, err := svc.SetLock(in(1), "sales", date("2025-09-30"), "  "); status(t, err) != http.StatusUnprocessableEntity {
		t.Fatalf("reopen without a reason: %v", err)
	}
	if _, err := svc.SetLock(in(1), "sales", date("2025-09-30"), "Faktur Oktober tertinggal"); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureOpen(in(1), "sales", *date("2025-10-15")); err != nil {
		t.Errorf("reopened October: %v", err)
	}
	recs := k.Audit.Records()
	last := recs[len(recs)-1]
	if last.Kind != kernel.AuditSecurity || last.Action != "lock.reopened" || last.Data["reason"] != "Faktur Oktober tertinggal" || last.Data["before"] != "2025-10-31" || last.Data["after"] != "2025-09-30" {
		t.Errorf("reopen audit = %+v", last)
	}
	if _, err := svc.SetLock(in(1), "sales", nil, "Buka semua untuk migrasi"); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.locks[1]["sales"]; ok {
		t.Error("removing the lock left a row")
	}
}

func TestLockRules(t *testing.T) {
	svc, _, _ := setup(t, authz{application.PermManage: all, application.PermUnlock: all})
	ctx := in(1)
	for _, d := range []string{"2026-01-02", "2026-03-01"} { // today, and the future
		if _, err := svc.SetLock(ctx, "sales", date(d), ""); status(t, err) != http.StatusUnprocessableEntity {
			t.Errorf("lock through %s: %v", d, err)
		}
	}
	if _, err := svc.SetLock(ctx, domain.All, date("2025-10-31"), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetLock(ctx, "sales", date("2025-09-30"), ""); status(t, err) != http.StatusUnprocessableEntity {
		t.Errorf("an area lock before the company-wide one: %v", err)
	}
	if _, err := svc.SetLock(ctx, "sales", date("2025-10-31"), ""); err != nil {
		t.Errorf("an area lock equal to the company-wide one: %v", err)
	}
	if _, err := svc.SetLock(ctx, "nope", date("2025-10-31"), ""); status(t, err) != http.StatusUnprocessableEntity {
		t.Errorf("an undeclared area: %v", err)
	}
	if _, err := svc.SetLock(ctx, "../x", nil, "alasan panjang"); status(t, err) != http.StatusNotFound {
		t.Errorf("a malformed area: %v", err)
	}
}

func TestOverview(t *testing.T) {
	svc, repo, _ := setup(t, authz{application.PermManage: all})
	_, _ = svc.SetLock(in(1), domain.All, date("2025-10-31"), "")
	_, _ = svc.SetLock(in(1), "payroll", date("2025-12-31"), "")
	_ = repo.PutLock(context.Background(), 1, domain.Lock{Area: "old", LockedThrough: *date("2024-12-31")})
	o, err := svc.Overview(in(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Areas) != 4 || o.Areas[0].Area != domain.All || o.Areas[1].Area != "sales" || o.Areas[3].Area != "old" || o.Areas[3].Declared {
		t.Fatalf("areas = %+v", o.Areas)
	}
	if o.Areas[1].Own != nil || *o.Areas[1].Effective != *date("2025-10-31") {
		t.Errorf("sales = %+v", o.Areas[1])
	}
	if *o.Areas[2].Effective != *date("2025-12-31") {
		t.Errorf("payroll = %+v", o.Areas[2])
	}
	if o.Today != *date("2026-01-02") || o.FiscalFrom != 1 || !o.CanClose || o.CanReopen {
		t.Errorf("today %v, fiscal %d, close %v, reopen %v", o.Today, o.FiscalFrom, o.CanClose, o.CanReopen)
	}
}

func TestFiscalYear(t *testing.T) {
	svc, _, k := setup(t, authz{application.PermManage: all})
	s, e, _ := svc.FiscalYear(in(1), *date("2026-03-15"))
	if s != *date("2026-01-01") || e != *date("2026-12-31") {
		t.Errorf("calendar year: %v – %v", s, e)
	}
	if err := svc.SetFiscalStart(in(1), 7); err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string][2]string{"2026-03-15": {"2025-07-01", "2026-06-30"}, "2026-07-01": {"2026-07-01", "2027-06-30"}, "2026-06-30": {"2025-07-01", "2026-06-30"}} {
		s, e, _ := svc.FiscalYear(kerneltest.In(context.Background(), 1, 0), *date(in))
		if s != *date(want[0]) || e != *date(want[1]) {
			t.Errorf("July year, %s: %v – %v", in, s, e)
		}
	}
	if s, _, _ := svc.FiscalYear(in(2), *date("2026-03-15")); s != *date("2026-01-01") {
		t.Errorf("company 2 keeps January: %v", s)
	}
	if err := svc.SetFiscalStart(in(1), 13); status(t, err) != http.StatusUnprocessableEntity {
		t.Errorf("month 13: %v", err)
	}
	if recs := k.Audit.Records(); len(recs) != 1 || recs[0].Kind != kernel.AuditSecurity {
		t.Errorf("audit records = %+v", recs)
	}
	svc, _, _ = setup(t, authz{})
	if err := svc.SetFiscalStart(in(1), 7); status(t, err) != http.StatusForbidden {
		t.Errorf("without manage: %v", err)
	}
}

func TestDeclare(t *testing.T) {
	svc, _, _ := setup(t, authz{})
	svc.Declare("sales", "Other label")
	if a := svc.Areas(); len(a) != 3 || a[1].Label != "Penjualan" {
		t.Errorf("areas = %+v", a)
	}
	defer func() {
		if recover() == nil {
			t.Error("Declare(\"Sales!\") did not panic")
		}
	}()
	svc.Declare("Sales!", "x")
}
