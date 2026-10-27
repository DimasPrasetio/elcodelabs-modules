package application_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/kernel/kerneltest"
	"__GO_MODULE__/internal/modules/numbering/application"
	"__GO_MODULE__/internal/modules/numbering/contracts"
	"__GO_MODULE__/internal/modules/numbering/domain"
	"__GO_MODULE__/internal/modules/numbering/ports"
	"__GO_MODULE__/internal/shared/apperror"
)

type memRepo struct {
	formats  map[int64]map[string]application.StoredFormat
	counters map[application.Series]int64
	issued   map[string]bool
}

func newMemRepo() *memRepo {
	return &memRepo{formats: map[int64]map[string]application.StoredFormat{}, counters: map[application.Series]int64{}, issued: map[string]bool{}}
}

func (m *memRepo) Given(_ context.Context, c int64, dt, number string) (bool, error) {
	return m.issued[fmt.Sprint(c, dt, number)], nil
}

func (m *memRepo) Record(_ context.Context, c int64, dt, number string) (bool, error) {
	k := fmt.Sprint(c, dt, number)
	if m.issued[k] {
		return false, nil
	}
	m.issued[k] = true
	return true, nil
}

func (m *memRepo) Formats(_ context.Context, c int64) (map[string]application.StoredFormat, error) {
	return m.formats[c], nil
}

func (m *memRepo) FormatOf(_ context.Context, c int64, dt string) (domain.Format, bool, error) {
	f, ok := m.formats[c][dt]
	return f.Format, ok, nil
}

func (m *memRepo) FormatOfShared(ctx context.Context, c int64, dt string) (domain.Format, bool, error) {
	return m.FormatOf(ctx, c, dt)
}

func (m *memRepo) PutFormat(_ context.Context, c int64, dt string, f domain.Format, by int64, at time.Time) error {
	if m.formats[c] == nil {
		m.formats[c] = map[string]application.StoredFormat{}
	}
	m.formats[c][dt] = application.StoredFormat{Format: f, UpdatedBy: by, UpdatedAt: at}
	return nil
}

func (m *memRepo) DeleteFormat(_ context.Context, c int64, dt string) error {
	delete(m.formats[c], dt)
	return nil
}

func (m *memRepo) Take(_ context.Context, s application.Series) (int64, error) {
	m.counters[s]++
	return m.counters[s], nil
}

func (m *memRepo) LastValue(_ context.Context, s application.Series) (int64, error) {
	return m.counters[s], nil
}

func (m *memRepo) LockSeries(_ context.Context, s application.Series) (int64, error) {
	return m.counters[s], nil
}

func (m *memRepo) Issued(_ context.Context, c int64, dt string) (bool, error) {
	for s, v := range m.counters {
		if s.CompanyID == c && s.DocType == dt && v > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (m *memRepo) SetLastValue(_ context.Context, s application.Series, v int64) error {
	m.counters[s] = v
	return nil
}

type units struct{}

func (units) CompanyCode(_ context.Context, id int64) (string, error) {
	return map[int64]string{1: "PST", 2: "ANK"}[id], nil
}

func (units) BranchCode(_ context.Context, id int64) (string, error) {
	if id == 99 {
		return "", errors.New("not found")
	}
	return map[int64]string{3: "BDG", 4: "SBY"}[id], nil
}

func (units) Branches(context.Context, int64) ([]ports.Branch, error) {
	return []ports.Branch{{ID: 3, Code: "BDG", Name: "Bandung", Active: true}, {ID: 4, Code: "SBY", Name: "Surabaya", Active: true}, {ID: 5, Code: "OLD", Active: false}}, nil
}

type authz map[string]kernel.Units

func (a authz) Units(_ context.Context, p string) (kernel.Units, error) { return a[p], nil }

type signedIn struct{}

func (signedIn) UserID(context.Context) (int64, bool) { return 7, true }

var all = kernel.Units{AllBranches: true}

func setup(t *testing.T, u ports.Units, perms authz) (*application.Service, *memRepo, *kerneltest.Kit) {
	t.Helper()
	k := kerneltest.Deps(t) // today: 2026-01-02 WIB
	repo := newMemRepo()
	svc := application.NewService(repo, k.Deps.Tx, u, k.Deps.Audit, perms, signedIn{}, k.Clock)
	svc.Declare("sales.invoice", "Faktur penjualan", domain.Format{Pattern: "INV/{YYYY}/{MM}/{SEQ:4}", Reset: domain.Monthly})
	svc.Declare("sales.order", "Pesanan", domain.Format{Pattern: "SO/{BRANCH}/{YY}/{SEQ:3}", Reset: domain.Yearly, PerBranch: true})
	return svc, repo, k
}

func day(y int, m time.Month, d int) kernel.Date { return kernel.Date{Year: y, Month: m, Day: d} }

func next(t *testing.T, k *kerneltest.Kit, svc *application.Service, ctx context.Context, dt string, d kernel.Date) string {
	t.Helper()
	var n string
	err := k.Deps.Tx.Do(ctx, func(ctx context.Context) (err error) {
		n, err = svc.Next(ctx, dt, d)
		return err
	})
	if err != nil {
		t.Fatalf("Next(%s): %v", dt, err)
	}
	return n
}

func status(t *testing.T, err error) int {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok {
		t.Fatalf("want *apperror.Error, got %v", err)
	}
	return e.Status
}

func TestNextCountsPerPeriodAndRefusesOutsideATransaction(t *testing.T) {
	svc, _, k := setup(t, units{}, authz{})
	ctx := kerneltest.In(context.Background(), 1, 0)
	if _, err := svc.Next(ctx, "sales.invoice", day(2026, 10, 1)); !errors.Is(err, contracts.ErrNotInTx) {
		t.Fatalf("outside d.Tx: %v", err)
	}
	for i, want := range []string{"INV/2026/10/0001", "INV/2026/10/0002"} {
		if got := next(t, k, svc, ctx, "sales.invoice", day(2026, 10, 5+i)); got != want {
			t.Errorf("#%d = %s; want %s", i+1, got, want)
		}
	}
	if got := next(t, k, svc, ctx, "sales.invoice", day(2026, 11, 1)); got != "INV/2026/11/0001" {
		t.Errorf("a new month = %s", got)
	}
	// Back-dated into October: October's own series.
	if got := next(t, k, svc, ctx, "sales.invoice", day(2026, 10, 31)); got != "INV/2026/10/0003" {
		t.Errorf("back-dated = %s", got)
	}
	if got := next(t, k, svc, kerneltest.In(context.Background(), 2, 0), "sales.invoice", day(2026, 10, 1)); got != "INV/2026/10/0001" {
		t.Errorf("company 2 = %s", got)
	}
	err := k.Deps.Tx.Do(ctx, func(ctx context.Context) error { _, err := svc.Next(ctx, "sales.nope", day(2026, 1, 1)); return err })
	if !errors.Is(err, application.ErrUnknownType) {
		t.Errorf("undeclared type: %v", err)
	}
}

func TestPerBranchSeriesAndCodes(t *testing.T) {
	svc, _, k := setup(t, units{}, authz{})
	bdg := kerneltest.In(context.Background(), 1, 3)
	sby := kerneltest.In(context.Background(), 1, 4)
	hq := kerneltest.In(context.Background(), 1, 0)
	if got := next(t, k, svc, bdg, "sales.order", day(2026, 3, 1)); got != "SO/BDG/26/001" {
		t.Errorf("Bandung = %s", got)
	}
	if got := next(t, k, svc, sby, "sales.order", day(2026, 3, 1)); got != "SO/SBY/26/001" {
		t.Errorf("Surabaya = %s", got)
	}
	if got := next(t, k, svc, bdg, "sales.order", day(2026, 3, 2)); got != "SO/BDG/26/002" {
		t.Errorf("Bandung #2 = %s", got)
	}
	if got := next(t, k, svc, hq, "sales.order", day(2026, 3, 2)); got != "SO/PST/26/001" {
		t.Errorf("company-wide series prints the company code = %s", got)
	}
	// Without the Units port, codes are IDs.
	svc2, _, k2 := setup(t, nil, authz{})
	if got := next(t, k2, svc2, bdg, "sales.order", day(2026, 3, 1)); got != "SO/3/26/001" {
		t.Errorf("without Units = %s", got)
	}
	// A code that cannot be found fails the posting rather than print a wrong number.
	err := k.Deps.Tx.Do(kerneltest.In(context.Background(), 1, 99), func(ctx context.Context) error {
		_, err := svc.Next(ctx, "sales.order", day(2026, 3, 1))
		return err
	})
	if err == nil {
		t.Error("an unknown branch code gave a number")
	}
}

func TestNextPassesOverNumbersGivenBefore(t *testing.T) {
	svc, repo, k := setup(t, units{}, authz{})
	ctx := kerneltest.In(context.Background(), 1, 3)
	// Numbers Bandung printed under a code another branch now has: SO/BDG/26/001 and 002.
	repo.issued[fmt.Sprint(int64(1), "sales.order", "SO/BDG/26/001")] = true
	repo.issued[fmt.Sprint(int64(1), "sales.order", "SO/BDG/26/002")] = true
	if got, _ := svc.Preview(ctx, "sales.order", day(2026, 3, 1)); got != "SO/BDG/26/003" {
		t.Errorf("Preview = %s; want the first number never given", got)
	}
	if got := next(t, k, svc, ctx, "sales.order", day(2026, 3, 1)); got != "SO/BDG/26/003" {
		t.Errorf("Next = %s; want the first number never given", got)
	}
}

func TestNextPassesOverRevisionsOfNumbersGiven(t *testing.T) {
	svc, repo, k := setup(t, units{}, authz{application.PermManage: all})
	ctx := kerneltest.In(context.Background(), 1, 0)
	// An earlier pattern gave X/121220/001, whose revision is X/121220/001-1; a later pattern
	// prints that very string as its first number.
	repo.issued[fmt.Sprint(int64(1), "sales.invoice", "X/121220/001")] = true
	if err := svc.SetFormat(ctx, "sales.invoice", domain.Format{Pattern: "X/{DD}{MM}{YY}/{SEQ:3}-1", Reset: domain.Daily}); err != nil {
		t.Fatal(err)
	}
	if got := next(t, k, svc, ctx, "sales.invoice", day(2020, 12, 12)); got != "X/121220/002-1" {
		t.Errorf("Next = %s; want past the revision of a number given", got)
	}
}

func TestPreviewTakesNothing(t *testing.T) {
	svc, repo, k := setup(t, units{}, authz{})
	ctx := kerneltest.In(context.Background(), 1, 0)
	for i := 0; i < 2; i++ {
		if got, err := svc.Preview(ctx, "sales.invoice", day(2026, 10, 1)); err != nil || got != "INV/2026/10/0001" {
			t.Fatalf("Preview = %s, %v", got, err)
		}
	}
	if len(repo.counters) != 0 {
		t.Fatalf("Preview created counters: %v", repo.counters)
	}
	next(t, k, svc, ctx, "sales.invoice", day(2026, 10, 1))
	if got, _ := svc.Preview(ctx, "sales.invoice", day(2026, 10, 1)); got != "INV/2026/10/0002" {
		t.Errorf("Preview after one = %s", got)
	}
}

func TestSetFormat(t *testing.T) {
	svc, _, k := setup(t, units{}, authz{application.PermManage: {BranchIDs: []int64{3}}})
	ctx := kerneltest.In(context.Background(), 1, 0)
	f := domain.Format{Pattern: "FJ-{YYYY}-{SEQ:5}", Reset: domain.Yearly}
	if err := svc.SetFormat(ctx, "sales.invoice", f); status(t, err) != http.StatusForbidden {
		t.Fatalf("manage in one branch only: %v", err)
	}
	svc, _, k = setup(t, units{}, authz{application.PermManage: all})
	if err := svc.SetFormat(ctx, "sales.invoice", domain.Format{Pattern: "FJ-{SEQ:5}", Reset: domain.Yearly}); status(t, err) != http.StatusUnprocessableEntity {
		t.Fatalf("a duplicate-making format: %v", err)
	}
	if err := svc.SetFormat(ctx, "sales.nope", f); status(t, err) != http.StatusNotFound {
		t.Fatalf("an undeclared type: %v", err)
	}
	if err := svc.SetFormat(ctx, "sales.invoice", f); err != nil {
		t.Fatal(err)
	}
	if got := next(t, k, svc, ctx, "sales.invoice", day(2026, 10, 1)); got != "FJ-2026-00001" {
		t.Errorf("after the change = %s", got)
	}
	if got := next(t, k, svc, kerneltest.In(context.Background(), 2, 0), "sales.invoice", day(2026, 10, 1)); got != "INV/2026/10/0001" {
		t.Errorf("company 2 keeps the default = %s", got)
	}
	if recs := k.Audit.Records(); len(recs) != 1 || recs[0].Action != "format.changed" {
		t.Errorf("audit = %+v", recs)
	}
	// Back to the monthly default would restart October at 0001: refused once a number was taken.
	if err := svc.ResetFormat(ctx, "sales.invoice"); status(t, err) != http.StatusConflict {
		t.Fatalf("reset after a number: %v", err)
	}
}

func TestRaiseNext(t *testing.T) {
	svc, repo, k := setup(t, units{}, authz{application.PermManage: {BranchIDs: []int64{3}}})
	hq := kerneltest.In(context.Background(), 1, 0)
	if err := svc.RaiseNext(hq, "sales.invoice", 0, 500, "pindah dari sistem lama"); status(t, err) != http.StatusForbidden {
		t.Fatalf("company-wide series with manage in one branch: %v", err)
	}
	if err := svc.RaiseNext(hq, "sales.order", 3, 120, "pindah dari sistem lama"); err != nil {
		t.Fatalf("own branch: %v", err)
	}
	if got := next(t, k, svc, kerneltest.In(context.Background(), 1, 3), "sales.order", day(2026, 1, 2)); got != "SO/BDG/26/120" {
		t.Errorf("after raising = %s", got)
	}
	if err := svc.RaiseNext(hq, "sales.order", 4, 50, "pindah dari sistem lama"); status(t, err) != http.StatusForbidden {
		t.Errorf("another branch: %v", err)
	}
	if err := svc.RaiseNext(hq, "sales.order", 77, 50, "pindah dari sistem lama"); status(t, err) != http.StatusUnprocessableEntity {
		t.Errorf("a branch of no one: %v", err)
	}

	svc, repo, k = setup(t, units{}, authz{application.PermManage: all})
	_ = repo
	if err := svc.RaiseNext(hq, "sales.invoice", 0, 500, ""); status(t, err) != http.StatusUnprocessableEntity {
		t.Errorf("no reason: %v", err)
	}
	if err := svc.RaiseNext(hq, "sales.invoice", 0, 500, "pindah dari sistem lama"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RaiseNext(hq, "sales.invoice", 0, 400, "turunkan"); status(t, err) != http.StatusUnprocessableEntity {
		t.Errorf("downward: %v", err)
	}
	if got := next(t, k, svc, hq, "sales.invoice", day(2026, 1, 2)); got != "INV/2026/01/0500" {
		t.Errorf("after raising = %s", got)
	}
	recs := k.Audit.Records()
	if len(recs) != 1 || recs[0].Kind != kernel.AuditSecurity || recs[0].Data["reason"] != "pindah dari sistem lama" {
		t.Errorf("audit = %+v", recs)
	}
}

func TestOverview(t *testing.T) {
	svc, _, k := setup(t, units{}, authz{application.PermRead: all, application.PermManage: {BranchIDs: []int64{3}}})
	ctx := kerneltest.In(context.Background(), 1, 0)
	next(t, k, svc, kerneltest.In(context.Background(), 1, 3), "sales.order", day(2026, 1, 2))
	types, today, manageFormats, err := svc.Overview(ctx)
	if err != nil || today != day(2026, 1, 2) || len(types) != 2 || manageFormats {
		t.Fatalf("Overview = %+v, %v, %v, %v", types, today, manageFormats, err)
	}
	if s := types[0].Series; len(s) != 1 || s[0].Next != "INV/2026/01/0001" {
		t.Errorf("invoice series = %+v", s)
	}
	s := types[1].Series // company-wide, then the two active branches
	if len(s) != 3 || s[0].Next != "SO/PST/26/001" || s[1].Next != "SO/BDG/26/002" || s[1].LastValue != 1 || s[2].Next != "SO/SBY/26/001" {
		t.Errorf("order series = %+v", s)
	}
	if s[0].Manageable || !s[1].Manageable || s[2].Manageable {
		t.Errorf("manageable = %+v", s)
	}
	// A reader in one branch sees the company's shared series and only their branch's own.
	svc2, _, _ := setup(t, units{}, authz{application.PermRead: {BranchIDs: []int64{4}}})
	types, _, _, _ = svc2.Overview(kerneltest.In(context.Background(), 1, 4))
	if len(types[0].Series) != 1 || len(types[1].Series) != 1 || types[1].Series[0].BranchID != 4 {
		t.Errorf("a branch reader sees %+v / %+v", types[0].Series, types[1].Series)
	}
}

func TestFormatChangesThatWouldRepeatNumbers(t *testing.T) {
	svc, _, k := setup(t, units{}, authz{application.PermManage: all})
	ctx := kerneltest.In(context.Background(), 1, 0)
	// Before any number: everything may change.
	if err := svc.SetFormat(ctx, "sales.invoice", domain.Format{Pattern: "INV/{YYYY}/{MM}/{SEQ:4}", Reset: domain.Yearly}); err != nil {
		t.Fatalf("before the first number: %v", err)
	}
	if err := svc.ResetFormat(ctx, "sales.invoice"); err != nil {
		t.Fatal(err)
	}
	next(t, k, svc, ctx, "sales.invoice", day(2026, 10, 1))
	for _, f := range []domain.Format{
		{Pattern: "INV/{YYYY}/{MM}/{SEQ:4}", Reset: domain.Yearly},
		{Pattern: "INV/{BRANCH}/{YYYY}/{MM}/{SEQ:4}", Reset: domain.Monthly, PerBranch: true},
	} {
		if err := svc.SetFormat(ctx, "sales.invoice", f); status(t, err) != http.StatusConflict {
			t.Errorf("after a number, %+v: %v", f, err)
		}
	}
	if err := svc.SetFormat(ctx, "sales.invoice", domain.Format{Pattern: "FJ-{YYYY}{MM}-{SEQ:5}", Reset: domain.Monthly}); err != nil {
		t.Errorf("a new pattern, same series: %v", err)
	}
	// Back to the default changes nothing about the series either: allowed.
	if err := svc.ResetFormat(ctx, "sales.invoice"); err != nil {
		t.Errorf("reset to the same series: %v", err)
	}
	// The other company has taken none: free to change.
	if err := svc.SetFormat(kerneltest.In(context.Background(), 2, 0), "sales.invoice", domain.Format{Pattern: "INV/{YYYY}/{SEQ:4}", Reset: domain.Yearly}); err != nil {
		t.Errorf("company 2: %v", err)
	}
}

func TestRaiseNextIsBounded(t *testing.T) {
	svc, _, _ := setup(t, units{}, authz{application.PermManage: all})
	ctx := kerneltest.In(context.Background(), 1, 0)
	if err := svc.RaiseNext(ctx, "sales.invoice", 0, 9223372036854775807, "migrasi data"); status(t, err) != http.StatusUnprocessableEntity {
		t.Errorf("next = max int64: %v", err)
	}
	if err := svc.RaiseNext(ctx, "sales.invoice", 0, domain.MaxSeq, "migrasi data"); status(t, err) != http.StatusUnprocessableEntity {
		t.Errorf("next = MaxSeq: %v", err)
	}
	if err := svc.RaiseNext(ctx, "sales.invoice", 0, domain.MaxRaise, "migrasi data"); err != nil {
		t.Errorf("next = MaxRaise: %v", err)
	}
}

func TestDeclareRefusesBadInput(t *testing.T) {
	svc, _, _ := setup(t, nil, authz{})
	for _, c := range []struct {
		name string
		f    domain.Format
	}{{"Invoice", domain.Format{Pattern: "{SEQ:3}", Reset: domain.Never}}, {"x.y", domain.Format{Pattern: "{MM}{SEQ:3}", Reset: domain.Yearly}}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Declare(%q, %+v) did not panic", c.name, c.f)
				}
			}()
			svc.Declare(c.name, "x", c.f)
		}()
	}
}
