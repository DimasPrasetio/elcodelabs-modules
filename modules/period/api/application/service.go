// Package application holds the period module's use cases: closing and reopening the books,
// refusing dates in a closed period, and the fiscal year.
package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/period/contracts"
	"__GO_MODULE__/internal/modules/period/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/validator"
)

// Permissions this module declares (period.module.go) and checks.
const (
	PermRead   = "period.read"
	PermManage = "period.manage"
	PermUnlock = "period.unlock"
)

// ErrUnknownArea is EnsureOpen asked about an area no module declared — a programming error.
var ErrUnknownArea = errors.New("period: area was never declared")

// Repository is the period_locks and period_settings tables, in the company passed.
type Repository interface {
	// HoldShared makes sure the company's settings row exists and holds it shared until the
	// transaction ends; HoldExclusive holds it exclusive. Every posting takes the first, every
	// lock change the second.
	HoldShared(ctx context.Context, companyID int64) error
	HoldExclusive(ctx context.Context, companyID int64) error
	// LocksOfArea is the area's lock and the company-wide one, by area; shared reads them with
	// a shared lock (the latest committed, whatever the transaction saw before).
	LocksOfArea(ctx context.Context, companyID int64, area string, shared bool) (map[string]kernel.Date, error)
	Locks(ctx context.Context, companyID int64) ([]domain.Lock, error)
	PutLock(ctx context.Context, companyID int64, l domain.Lock) error
	DeleteLock(ctx context.Context, companyID int64, area string) error
	// FiscalStart is the month the fiscal year starts (1 when never set).
	FiscalStart(ctx context.Context, companyID int64) (int, error)
	SetFiscalStart(ctx context.Context, companyID int64, month int, at time.Time) error
}

// Transactor runs fn in one transaction and tells whether ctx is in one — the kernel's d.Tx.
type Transactor interface {
	Do(ctx context.Context, fn func(ctx context.Context) error, opts ...kernel.TxOption) error
	InTx(ctx context.Context) bool
}

// Auditor records changes — the kernel's d.Audit.
type Auditor interface {
	Record(ctx context.Context, e kernel.AuditEntry)
}

// Authorizer answers how far the signed-in user's permissions reach — the kernel's d.Authz.
type Authorizer interface {
	Units(ctx context.Context, permission string) (kernel.Units, error)
}

// Identity is who is signed in — the kernel's d.Auth.
type Identity interface {
	UserID(ctx context.Context) (int64, bool)
}

// Service is the period module's use cases.
type Service struct {
	repo  Repository
	tx    Transactor
	audit Auditor
	authz Authorizer
	auth  Identity
	clock kernel.Clock

	mu    sync.RWMutex
	areas []Area // declared, in order; All first
}

// Area is a part of the books that can be closed on its own.
type Area struct {
	Name  string
	Label string
}

func NewService(repo Repository, tx Transactor, audit Auditor, authz Authorizer, auth Identity, clock kernel.Clock) *Service {
	return &Service{repo: repo, tx: tx, audit: audit, authz: authz, auth: auth, clock: clock,
		areas: []Area{{Name: domain.All, Label: "Semua area"}}}
}

// Declare registers an area. A name that is not lowercase letters, digits, and . _ - is a
// programming error and panics when the app starts; declaring a name twice keeps the first.
func (s *Service) Declare(area, label string) {
	if !domain.AreaName.MatchString(area) {
		panic(fmt.Sprintf("period: area %q must be lowercase letters, digits, and . _ - (at most 50)", area))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.areas {
		if a.Name == area {
			return
		}
	}
	s.areas = append(s.areas, Area{Name: area, Label: strings.TrimSpace(label)})
}

// Areas lists the declared areas, the company-wide one first.
func (s *Service) Areas() []Area {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Area(nil), s.areas...)
}

func (s *Service) label(area string) (string, bool) {
	for _, a := range s.Areas() {
		if a.Name == area {
			return a.Label, true
		}
	}
	return area, false
}

// EnsureOpen refuses a date in a closed period of area (see contracts.PeriodClient).
func (s *Service) EnsureOpen(ctx context.Context, area string, date kernel.Date) error {
	label, known := s.label(area)
	if !known {
		return fmt.Errorf("%w: %q", ErrUnknownArea, area)
	}
	company := kernel.ScopeFrom(ctx).CompanyID
	inTx := s.tx.InTx(ctx)
	if inTx {
		if err := s.repo.HoldShared(ctx, company); err != nil {
			return err
		}
	}
	locks, err := s.repo.LocksOfArea(ctx, company, area, inTx)
	if err != nil {
		return err
	}
	if through, closed := domain.IsClosed(locks, area, date); closed {
		what := "Buku"
		if area != domain.All {
			what = "Buku " + label
		}
		return &contracts.ClosedError{Area: area, Through: through,
			Msg: fmt.Sprintf("%s sudah ditutup sampai %s — tanggal %s ada di periode yang ditutup", what, domain.FormatDate(through), domain.FormatDate(date))}
	}
	return nil
}

// FiscalYear is the fiscal year date falls in, for the company ctx works in.
func (s *Service) FiscalYear(ctx context.Context, date kernel.Date) (kernel.Date, kernel.Date, error) {
	m, err := s.repo.FiscalStart(ctx, kernel.ScopeFrom(ctx).CompanyID)
	if err != nil {
		return kernel.Date{}, kernel.Date{}, err
	}
	start, end := domain.FiscalYear(m, date)
	return start, end, nil
}

// AreaLock is one area as the settings page shows it.
type AreaLock struct {
	Area      string
	Label     string
	Declared  bool         // false: a lock stored for an area no installed module declares
	Own       *domain.Lock // nil: no lock of its own
	Effective *kernel.Date // what actually closes it: the later of its own and the company-wide
}

// Overview is the settings page: every area with its locks, and the fiscal year.
type Overview struct {
	Areas      []AreaLock
	FiscalFrom int
	Today      kernel.Date
	CanClose   bool // period.manage across the company: close more, change the fiscal year
	CanReopen  bool // period.unlock across the company
}

// Overview lists the areas and their locks in the company ctx works in.
func (s *Service) Overview(ctx context.Context) (Overview, error) {
	company := kernel.ScopeFrom(ctx).CompanyID
	stored, err := s.repo.Locks(ctx, company)
	if err != nil {
		return Overview{}, err
	}
	m, err := s.repo.FiscalStart(ctx, company)
	if err != nil {
		return Overview{}, err
	}
	byArea := map[string]domain.Lock{}
	dates := map[string]kernel.Date{}
	for _, l := range stored {
		byArea[l.Area] = l
		dates[l.Area] = l.LockedThrough
	}
	out := Overview{FiscalFrom: m, Today: kernel.Today(ctx, s.clock)}
	for perm, can := range map[string]*bool{PermManage: &out.CanClose, PermUnlock: &out.CanReopen} {
		units, err := s.authz.Units(ctx, perm)
		if err != nil {
			return Overview{}, err
		}
		*can = units.AllBranches
	}
	add := func(name, label string, declared bool) {
		a := AreaLock{Area: name, Label: label, Declared: declared}
		if l, ok := byArea[name]; ok {
			a.Own = &l
		}
		if through, ok := domain.Closing(dates, name); ok {
			a.Effective = &through
		}
		out.Areas = append(out.Areas, a)
	}
	for _, a := range s.Areas() {
		add(a.Name, a.Label, true)
		delete(byArea, a.Name)
	}
	var orphans []string
	for name := range byArea {
		orphans = append(orphans, name)
	}
	sort.Strings(orphans)
	for _, name := range orphans {
		add(name, name, false)
	}
	return out, nil
}

// SetLock moves area's lock to through, or removes it (nil). Closing more needs period.manage,
// reopening — an earlier date, or no lock — needs period.unlock and a reason, recorded as a
// security event; both across the whole company. The lock changes under the company's
// exclusive hold, so it never moves under a posting in flight.
func (s *Service) SetLock(ctx context.Context, area string, through *kernel.Date, reason string) (*domain.Lock, error) {
	label, known := s.label(area)
	if !known {
		if !domain.AreaName.MatchString(area) {
			return nil, apperror.NotFound("Area tutup buku tidak dikenal")
		}
		if through != nil { // an area no module declares any more: it may only be reopened
			return nil, apperror.Validation([]validator.FieldError{{Field: "area", Message: "Area ini tidak dipakai modul mana pun — kuncinya hanya bisa dihapus"}})
		}
	}
	actor, _ := s.auth.UserID(ctx)
	company := kernel.ScopeFrom(ctx).CompanyID
	today := kernel.Today(ctx, s.clock)
	if through != nil && !domain.Before(*through, today) {
		return nil, apperror.Validation([]validator.FieldError{{Field: "locked_through", Message: "Tanggal kunci harus sebelum hari ini (" + domain.FormatDate(today) + ")"}})
	}
	reason = strings.TrimSpace(reason)

	var result *domain.Lock
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.HoldExclusive(ctx, company); err != nil {
			return err
		}
		locks, err := s.repo.LocksOfArea(ctx, company, area, true)
		if err != nil {
			return err
		}
		current, has := locks[area]
		all, hasAll := locks[domain.All]
		if area != domain.All && through != nil && hasAll && domain.Before(*through, all) {
			return apperror.Validation([]validator.FieldError{{Field: "locked_through", Message: "Semua area sudah ditutup sampai " + domain.FormatDate(all) + " — pilih tanggal itu atau setelahnya"}})
		}
		reopen := (through == nil && has) || (through != nil && has && domain.Before(*through, current))
		closeMore := through != nil && (!has || domain.Before(current, *through))
		if !reopen && !closeMore {
			if has {
				result = &domain.Lock{Area: area, LockedThrough: current}
			}
			return nil // nothing changes
		}
		perm, deny := PermManage, "Menutup buku butuh izin period.manage di seluruh perusahaan"
		if reopen {
			perm, deny = PermUnlock, "Membuka kembali buku yang sudah ditutup butuh izin period.unlock di seluruh perusahaan"
		}
		if units, err := s.authz.Units(ctx, perm); err != nil {
			return err
		} else if !units.AllBranches {
			return apperror.Forbidden(deny)
		}
		if reopen {
			if n := utf8.RuneCountInString(reason); n < 5 || n > 500 {
				return apperror.Validation([]validator.FieldError{{Field: "reason", Message: "Alasan membuka kembali wajib diisi, 5 sampai 500 karakter"}})
			}
		}

		now := s.clock.Now()
		before, after := "", ""
		if has {
			before = current.String()
		}
		if through == nil {
			if err := s.repo.DeleteLock(ctx, company, area); err != nil {
				return err
			}
		} else {
			l := domain.Lock{Area: area, LockedThrough: *through, UpdatedBy: actor, UpdatedAt: now}
			if err := s.repo.PutLock(ctx, company, l); err != nil {
				return err
			}
			result, after = &l, through.String()
		}
		e := kernel.AuditEntry{Module: "period", Target: "period:" + area, Data: map[string]any{"area": area, "before": before, "after": after}}
		if reopen {
			e.Kind, e.Action = kernel.AuditSecurity, "lock.reopened"
			e.Summary = "Membuka kembali buku " + label + ": " + describe(before) + " → " + describe(after)
			e.Data["reason"] = reason
		} else {
			e.Action = "lock.closed"
			e.Summary = "Menutup buku " + label + " sampai " + describe(after)
		}
		s.audit.Record(ctx, e)
		return nil
	})
	return result, err
}

func describe(iso string) string {
	if iso == "" {
		return "tanpa kunci"
	}
	d, err := domain.ParseDate(iso)
	if err != nil {
		return iso
	}
	return domain.FormatDate(d)
}

// SetFiscalStart sets the month the company's fiscal year starts: period.manage across the
// company.
func (s *Service) SetFiscalStart(ctx context.Context, month int) error {
	if month < 1 || month > 12 {
		return apperror.Validation([]validator.FieldError{{Field: "fiscal_year_start_month", Message: "Pilih bulan 1 sampai 12"}})
	}
	if units, err := s.authz.Units(ctx, PermManage); err != nil {
		return err
	} else if !units.AllBranches {
		return apperror.Forbidden("Mengubah tahun buku butuh izin period.manage di seluruh perusahaan")
	}
	company := kernel.ScopeFrom(ctx).CompanyID
	return s.tx.Do(ctx, func(ctx context.Context) error {
		before, err := s.repo.FiscalStart(ctx, company)
		if err != nil {
			return err
		}
		if before == month {
			return nil
		}
		if err := s.repo.SetFiscalStart(ctx, company, month, s.clock.Now()); err != nil {
			return err
		}
		// A security record like a reopen: moving the year's start moves which months a
		// closing or a report by fiscal year covers.
		s.audit.Record(ctx, kernel.AuditEntry{Kind: kernel.AuditSecurity, Module: "period", Action: "fiscal_year.changed", Target: "period:fiscal-year",
			Summary: fmt.Sprintf("Mengubah awal tahun buku dari bulan %d ke bulan %d", before, month), Data: map[string]any{"before": before, "after": month}})
		return nil
	})
}
