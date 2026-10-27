// Package application holds the numbering module's use cases: declared document types, their
// formats per company, and taking numbers without gaps.
package application

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/numbering/contracts"
	"__GO_MODULE__/internal/modules/numbering/domain"
	"__GO_MODULE__/internal/modules/numbering/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/validator"
)

// Permissions this module declares (numbering.module.go) and checks.
const (
	PermRead   = "numbering.read"
	PermManage = "numbering.manage"
)

// ErrUnknownType is a document type no module declared — a programming error.
var ErrUnknownType = errors.New("numbering: document type was never declared")

// Series is one counter: a company's document type, in a branch (0: not per branch), in a reset
// period.
type Series struct {
	CompanyID int64
	DocType   string
	BranchID  int64
	PeriodKey string
}

// StoredFormat is a company's own format for a type.
type StoredFormat struct {
	Format    domain.Format
	UpdatedBy int64
	UpdatedAt time.Time
}

// Repository is the num_formats and num_counters tables.
type Repository interface {
	Formats(ctx context.Context, companyID int64) (map[string]StoredFormat, error)
	FormatOf(ctx context.Context, companyID int64, docType string) (domain.Format, bool, error)
	// FormatOfShared is FormatOf holding the row shared until the transaction ends.
	FormatOfShared(ctx context.Context, companyID int64, docType string) (domain.Format, bool, error)
	PutFormat(ctx context.Context, companyID int64, docType string, f domain.Format, by int64, at time.Time) error
	DeleteFormat(ctx context.Context, companyID int64, docType string) error
	// Take adds 1 to the series (creating it at 1) and returns the new value; the row stays
	// locked until the transaction ends.
	Take(ctx context.Context, s Series) (int64, error)
	// LastValue is the series' last value, 0 when it has none yet.
	LastValue(ctx context.Context, s Series) (int64, error)
	// LockSeries creates the series at 0 when missing, locks it, and returns its last value.
	LockSeries(ctx context.Context, s Series) (int64, error)
	SetLastValue(ctx context.Context, s Series, v int64) error
	// Issued reports whether any number of docType was taken in the company, holding the
	// counters shared until the transaction ends.
	Issued(ctx context.Context, companyID int64, docType string) (bool, error)
	// Record keeps number as given; false when it was given before (by any series).
	Record(ctx context.Context, companyID int64, docType, number string) (bool, error)
	// Given reports whether number was given before, without keeping anything.
	Given(ctx context.Context, companyID int64, docType, number string) (bool, error)
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

// DocType is a declared document type.
type DocType struct {
	Name    string
	Label   string
	Default domain.Format
}

// Service is the numbering module's use cases.
type Service struct {
	repo  Repository
	tx    Transactor
	units ports.Units // nil: codes print as IDs
	audit Auditor
	authz Authorizer
	auth  Identity
	clock kernel.Clock

	mu    sync.RWMutex
	types []DocType
}

func NewService(repo Repository, tx Transactor, units ports.Units, audit Auditor, authz Authorizer, auth Identity, clock kernel.Clock) *Service {
	return &Service{repo: repo, tx: tx, units: units, audit: audit, authz: authz, auth: auth, clock: clock}
}

// Declare registers a document type. A bad name or default format is a programming error and
// panics when the app starts; declaring a type twice keeps the first.
func (s *Service) Declare(docType, label string, def domain.Format) {
	if len(docType) > 100 || !domain.DocType.MatchString(docType) {
		panic(fmt.Sprintf("numbering: document type %q must be <module>.<thing>, lowercase", docType))
	}
	if err := def.Validate(); err != nil {
		panic(fmt.Sprintf("numbering: default format of %q: %v", docType, err))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.types {
		if t.Name == docType {
			return
		}
	}
	s.types = append(s.types, DocType{Name: docType, Label: strings.TrimSpace(label), Default: def})
}

// Types lists the declared document types, in the order declared.
func (s *Service) Types() []DocType {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]DocType(nil), s.types...)
}

func (s *Service) declared(docType string) (DocType, bool) {
	for _, t := range s.Types() {
		if t.Name == docType {
			return t, true
		}
	}
	return DocType{}, false
}

// format is the company's format for docType, or the declared default.
func (s *Service) format(ctx context.Context, company int64, docType string) (domain.Format, error) {
	t, ok := s.declared(docType)
	if !ok {
		return domain.Format{}, fmt.Errorf("%w: %q", ErrUnknownType, docType)
	}
	f, custom, err := s.repo.FormatOf(ctx, company, docType)
	if err != nil {
		return domain.Format{}, err
	}
	if !custom {
		return t.Default, nil
	}
	return f, nil
}

// lockedFormat is format read with a shared lock on the company's format row (or, under the
// default REPEATABLE READ, the gap where it would be).
func (s *Service) lockedFormat(ctx context.Context, company int64, docType string) (domain.Format, error) {
	t, ok := s.declared(docType)
	if !ok {
		return domain.Format{}, fmt.Errorf("%w: %q", ErrUnknownType, docType)
	}
	f, custom, err := s.repo.FormatOfShared(ctx, company, docType)
	if err != nil || !custom {
		return t.Default, err
	}
	return f, nil
}

func seriesOf(company, branch int64, docType string, f domain.Format, date kernel.Date) Series {
	if !f.PerBranch {
		branch = 0
	}
	return Series{CompanyID: company, DocType: docType, BranchID: branch, PeriodKey: domain.PeriodKey(f.Reset, date)}
}

// codes are what {COMPANY} and {BRANCH} print, looked up only when the pattern uses them. A
// company-wide series (branch 0) prints the company's code for {BRANCH}.
func (s *Service) codes(ctx context.Context, f domain.Format, company, branch int64) (domain.Codes, error) {
	var c domain.Codes
	needCompany := strings.Contains(f.Pattern, "{COMPANY}") || (branch == 0 && strings.Contains(f.Pattern, "{BRANCH}"))
	if needCompany {
		c.Company = strconv.FormatInt(company, 10)
		if s.units != nil {
			code, err := s.units.CompanyCode(ctx, company)
			if err != nil {
				return c, fmt.Errorf("numbering: company %d code: %w", company, err)
			}
			c.Company = code
		}
	}
	if strings.Contains(f.Pattern, "{BRANCH}") {
		if branch == 0 {
			c.Branch = c.Company
		} else {
			c.Branch = strconv.FormatInt(branch, 10)
			if s.units != nil {
				code, err := s.units.BranchCode(ctx, branch)
				if err != nil {
					return c, fmt.Errorf("numbering: branch %d code: %w", branch, err)
				}
				c.Branch = code
			}
		}
	}
	return c, nil
}

// Next takes the next number (see contracts.NumberingClient).
func (s *Service) Next(ctx context.Context, docType string, date kernel.Date) (string, error) {
	if !s.tx.InTx(ctx) {
		return "", contracts.ErrNotInTx
	}
	sc := kernel.ScopeFrom(ctx)
	f, err := s.format(ctx, sc.CompanyID, docType)
	if err != nil {
		return "", err
	}
	series := seriesOf(sc.CompanyID, sc.BranchID, docType, f, date)
	codes, err := s.codes(ctx, f, sc.CompanyID, series.BranchID)
	if err != nil {
		return "", err
	}
	// A value whose number was printed before — by a branch that had this code, or a series a
	// format change left behind — is passed over: the next one is taken, in the same lock.
	for range maxSkips {
		v, err := s.repo.Take(ctx, series)
		if err != nil {
			return "", err
		}
		if v > domain.MaxSeq {
			return "", apperror.Conflict(fmt.Sprintf("Urutan nomor %s sudah mencapai batas %d", docType, int64(domain.MaxSeq)))
		}
		number := f.Render(date, v, codes)
		if len(number) > domain.MaxNumber {
			return "", apperror.Conflict(fmt.Sprintf("Nomor %s lebih dari %d karakter — persingkat kode cabang atau pola", number, domain.MaxNumber))
		}
		if revision, err := s.revisionOfGiven(ctx, sc.CompanyID, docType, number); err != nil {
			return "", err
		} else if revision {
			continue
		}
		fresh, err := s.repo.Record(ctx, sc.CompanyID, docType, number)
		if err != nil {
			return "", err
		}
		if fresh {
			return number, nil
		}
	}
	return "", apperror.Conflict(fmt.Sprintf("%d nomor berturut-turut untuk %s sudah pernah dipakai — naikkan nomor berikutnya di Pengaturan › Penomoran", maxSkips, docType))
}

// maxSkips bounds how many already-given numbers one Next passes over.
const maxSkips = 1000

// revisionSuffix is how docstate numbers a revision: the original's number and "-1", "-2", ….
var revisionSuffix = regexp.MustCompile(`^(.+)-[1-9][0-9]*$`)

// revisionOfGiven reports whether number could be a revision of a number given before — after
// a pattern change, "X/121220/001-1" may be both. Revisions are numbered by the consuming
// module, not here, so the original stands in for them.
func (s *Service) revisionOfGiven(ctx context.Context, company int64, docType, number string) (bool, error) {
	m := revisionSuffix.FindStringSubmatch(number)
	if m == nil {
		return false, nil
	}
	return s.repo.Given(ctx, company, docType, m[1])
}

// Preview is what Next would return now, without taking it.
func (s *Service) Preview(ctx context.Context, docType string, date kernel.Date) (string, error) {
	sc := kernel.ScopeFrom(ctx)
	f, err := s.format(ctx, sc.CompanyID, docType)
	if err != nil {
		return "", err
	}
	return s.preview(ctx, f, seriesOf(sc.CompanyID, sc.BranchID, docType, f, date), date)
}

func (s *Service) preview(ctx context.Context, f domain.Format, series Series, date kernel.Date) (string, error) {
	codes, err := s.codes(ctx, f, series.CompanyID, series.BranchID)
	if err != nil {
		return "", err
	}
	last, err := s.repo.LastValue(ctx, series)
	if err != nil {
		return "", err
	}
	// What Next would give: past numbers already given, like Next.
	for v := last + 1; v <= last+maxSkips; v++ {
		number := f.Render(date, v, codes)
		given, err := s.repo.Given(ctx, series.CompanyID, series.DocType, number)
		if err == nil && !given {
			given, err = s.revisionOfGiven(ctx, series.CompanyID, series.DocType, number)
		}
		if err != nil || !given {
			return number, err
		}
	}
	return f.Render(date, last+1, codes), nil
}

// SeriesPreview is the next number of one series today.
type SeriesPreview struct {
	BranchID   int64
	BranchName string // "" for the company-wide series
	LastValue  int64
	Next       string
	Manageable bool // the signed-in user may raise its next number
}

// TypeView is a document type as the settings page shows it.
type TypeView struct {
	DocType
	Format    domain.Format
	Custom    bool // the company changed the format
	UpdatedBy int64
	UpdatedAt time.Time
	Series    []SeriesPreview // the ones within the signed-in user's reach
}

// Overview lists the declared types with their formats and today's next numbers, in the
// company ctx works in: per active branch for a per-branch type, only the series within the
// user's numbering.read. manageFormats says whether the user may change formats.
func (s *Service) Overview(ctx context.Context) (types []TypeView, today kernel.Date, manageFormats bool, err error) {
	company := kernel.ScopeFrom(ctx).CompanyID
	today = kernel.Today(ctx, s.clock)
	read, err := s.authz.Units(ctx, PermRead)
	if err != nil {
		return nil, today, false, err
	}
	manage, err := s.authz.Units(ctx, PermManage)
	if err != nil {
		return nil, today, false, err
	}
	types, err = s.overview(ctx, company, today, read, manage)
	return types, today, manage.AllBranches, err
}

func (s *Service) overview(ctx context.Context, company int64, today kernel.Date, read, manage kernel.Units) ([]TypeView, error) {
	stored, err := s.repo.Formats(ctx, company)
	if err != nil {
		return nil, err
	}
	var branches []ports.Branch
	if s.units != nil {
		all, err := s.units.Branches(ctx, company)
		if err != nil {
			return nil, err
		}
		for _, b := range all {
			if b.Active {
				branches = append(branches, b)
			}
		}
	}
	var out []TypeView
	for _, t := range s.Types() {
		v := TypeView{DocType: t, Format: t.Default}
		if sf, ok := stored[t.Name]; ok {
			v.Format, v.Custom, v.UpdatedBy, v.UpdatedAt = sf.Format, true, sf.UpdatedBy, sf.UpdatedAt
		}
		units := []ports.Branch{{}}
		if v.Format.PerBranch {
			units = append(units, branches...)
			if s.units == nil && kernel.ScopeFrom(ctx).BranchID != 0 {
				units = append(units, ports.Branch{ID: kernel.ScopeFrom(ctx).BranchID})
			}
		}
		for _, b := range units {
			// A type counted for the whole company is everyone's; a branch's series only for
			// those whose reading reaches that branch.
			if v.Format.PerBranch && !read.Includes(b.ID) {
				continue
			}
			series := seriesOf(company, b.ID, t.Name, v.Format, today)
			next, err := s.preview(ctx, v.Format, series, today)
			if err != nil {
				return nil, err
			}
			last, _ := s.repo.LastValue(ctx, series)
			v.Series = append(v.Series, SeriesPreview{BranchID: b.ID, BranchName: b.Name, LastValue: last, Next: next, Manageable: manage.Includes(b.ID)})
		}
		out = append(out, v)
	}
	return out, nil
}

// Sample is a format rendered for today at counter 1 — for the edit form, before saving.
func (s *Service) Sample(ctx context.Context, f domain.Format) (string, error) {
	if err := f.Validate(); err != nil {
		return "", apperror.Validation([]validator.FieldError{{Field: "pattern", Message: err.Error()}})
	}
	sc := kernel.ScopeFrom(ctx)
	codes, err := s.codes(ctx, f, sc.CompanyID, sc.BranchID)
	if err != nil {
		return "", err
	}
	return f.Render(kernel.Today(ctx, s.clock), 1, codes), nil
}

func (s *Service) mayManage(ctx context.Context, branch int64, deny string) error {
	units, err := s.authz.Units(ctx, PermManage)
	if err != nil {
		return err
	}
	if !units.Includes(branch) { // branch 0, the company itself, needs every branch
		return apperror.Forbidden(deny)
	}
	return nil
}

func formatData(f domain.Format) map[string]any {
	return map[string]any{"pattern": f.Pattern, "reset": string(f.Reset), "per_branch": f.PerBranch}
}

// SetFormat changes the company's format for docType: numbering.manage across the company.
// Numbers already given keep theirs; a changed reset or per-branch choice starts new series.
func (s *Service) SetFormat(ctx context.Context, docType string, f domain.Format) error {
	t, ok := s.declared(docType)
	if !ok {
		return apperror.NotFound("Jenis dokumen tidak dikenal")
	}
	f.Pattern = strings.TrimSpace(f.Pattern)
	if err := f.Validate(); err != nil {
		return apperror.Validation([]validator.FieldError{{Field: "pattern", Message: err.Error()}})
	}
	if err := s.mayManage(ctx, 0, "Mengubah format nomor butuh izin numbering.manage di seluruh perusahaan"); err != nil {
		return err
	}
	actor, _ := s.auth.UserID(ctx)
	company := kernel.ScopeFrom(ctx).CompanyID
	return s.tx.Do(ctx, func(ctx context.Context) error {
		before, err := s.format(ctx, company, docType)
		if err != nil {
			return err
		}
		if before == f {
			return nil
		}
		if err := s.keepsSeries(ctx, company, docType, before, f); err != nil {
			return err
		}
		if err := s.repo.PutFormat(ctx, company, docType, f, actor, s.clock.Now()); err != nil {
			return err
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "numbering", Action: "format.changed", Target: "numbering:" + docType,
			Summary: "Mengubah format nomor " + t.Label + " menjadi " + f.Pattern, Data: map[string]any{"before": formatData(before), "after": formatData(f)}})
		return nil
	})
}

// keepsSeries refuses a change of reset or per-branch counting once a number was taken: the new
// series would start again from 1 and print numbers already given (a monthly INV/2026/10/0001
// becoming a yearly one prints INV/2026/10/0001 again). The pattern itself may still change.
func (s *Service) keepsSeries(ctx context.Context, company int64, docType string, before, after domain.Format) error {
	if before.Reset == after.Reset && before.PerBranch == after.PerBranch {
		return nil
	}
	issued, err := s.repo.Issued(ctx, company, docType)
	if err != nil {
		return err
	}
	if issued {
		return apperror.Conflict("Nomor sudah pernah dipakai: cara mulai ulang dan urutan per cabang tidak bisa diubah lagi, karena urutan baru akan mengulang nomor lama. Polanya masih bisa diubah.")
	}
	return nil
}

// ResetFormat brings docType back to its declared format.
func (s *Service) ResetFormat(ctx context.Context, docType string) error {
	t, ok := s.declared(docType)
	if !ok {
		return apperror.NotFound("Jenis dokumen tidak dikenal")
	}
	if err := s.mayManage(ctx, 0, "Mengubah format nomor butuh izin numbering.manage di seluruh perusahaan"); err != nil {
		return err
	}
	company := kernel.ScopeFrom(ctx).CompanyID
	return s.tx.Do(ctx, func(ctx context.Context) error {
		before, custom, err := s.repo.FormatOf(ctx, company, docType)
		if err != nil || !custom {
			return err
		}
		if err := s.keepsSeries(ctx, company, docType, before, t.Default); err != nil {
			return err
		}
		if err := s.repo.DeleteFormat(ctx, company, docType); err != nil {
			return err
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "numbering", Action: "format.changed", Target: "numbering:" + docType,
			Summary: "Mengembalikan format nomor " + t.Label + " ke bawaan " + t.Default.Pattern, Data: map[string]any{"before": formatData(before), "after": formatData(t.Default)}})
		return nil
	})
}

// RaiseNext makes next the next number of docType's series for today in branch (0 for a series
// that is not per branch) — when moving from another system, so new numbers do not repeat old
// ones. Only upward, with a reason; numbering.manage reaching that branch; a security record.
func (s *Service) RaiseNext(ctx context.Context, docType string, branch, next int64, reason string) error {
	t, ok := s.declared(docType)
	if !ok {
		return apperror.NotFound("Jenis dokumen tidak dikenal")
	}
	if next < 1 || next > domain.MaxRaise {
		return apperror.Validation([]validator.FieldError{{Field: "next", Message: fmt.Sprintf("Nomor berikutnya antara 1 dan %d", int64(domain.MaxRaise))}})
	}
	reason = strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(reason); n < 5 || n > 500 {
		return apperror.Validation([]validator.FieldError{{Field: "reason", Message: "Alasan wajib diisi, 5 sampai 500 karakter"}})
	}
	sc := kernel.ScopeFrom(ctx)
	f, err := s.format(ctx, sc.CompanyID, docType)
	if err != nil {
		return err
	}
	if !f.PerBranch {
		branch = 0
	} else if branch != 0 && s.units != nil {
		bs, err := s.units.Branches(ctx, sc.CompanyID)
		if err != nil {
			return err
		}
		found := false
		for _, b := range bs {
			found = found || b.ID == branch
		}
		if !found {
			return apperror.Validation([]validator.FieldError{{Field: "branch_id", Message: "Cabang tidak ada di perusahaan ini"}})
		}
	}
	if err := s.mayManage(ctx, branch, "Mengubah nomor berikutnya butuh izin numbering.manage di cabang itu"); err != nil {
		return err
	}
	today := kernel.Today(ctx, s.clock)
	return s.tx.Do(ctx, func(ctx context.Context) error {
		// The format again, inside the transaction and held shared: the series raised is the one
		// in use, and stays so until the commit (a format change waits).
		now, err := s.lockedFormat(ctx, sc.CompanyID, docType)
		if err != nil {
			return err
		}
		if now.Reset != f.Reset || now.PerBranch != f.PerBranch {
			return apperror.Conflict("Format nomor baru saja diubah — muat ulang")
		}
		series := seriesOf(sc.CompanyID, branch, docType, now, today)
		last, err := s.repo.LockSeries(ctx, series)
		if err != nil {
			return err
		}
		if next-1 < last {
			return apperror.Validation([]validator.FieldError{{Field: "next", Message: fmt.Sprintf("Nomor berikutnya hanya bisa dinaikkan — sekarang %d", last+1)}})
		}
		if next-1 == last {
			return nil
		}
		if err := s.repo.SetLastValue(ctx, series, next-1); err != nil {
			return err
		}
		s.audit.Record(ctx, kernel.AuditEntry{Kind: kernel.AuditSecurity, Module: "numbering", Action: "counter.raised", Target: "numbering:" + docType,
			Summary: fmt.Sprintf("Menaikkan nomor berikutnya %s dari %d ke %d", t.Label, last+1, next),
			Data:    map[string]any{"branch_id": branch, "period": series.PeriodKey, "before": last + 1, "after": next, "reason": reason}})
		return nil
	})
}
