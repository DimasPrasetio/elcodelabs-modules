// Package application holds the organization module's use cases: companies, branches, placing
// users, and resolving the scope every request works in.
package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/organization/domain"
	"__GO_MODULE__/internal/modules/organization/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/validator"
)

// Errors a Repository reports.
var (
	ErrNotFound  = errors.New("not found")
	ErrCodeTaken = errors.New("code already in use")
)

// UnitPlacement is a placement as the resolver and the picker see it: with its company's and
// branch's names and time zones. Only active companies and branches appear.
type UnitPlacement struct {
	CompanyID       int64
	BranchID        int64
	IsDefault       bool
	CompanyCode     string
	CompanyName     string
	CompanyTimezone string
	BranchCode      string
	BranchName      string
	BranchTimezone  string // "" — the company's
}

// Repository is the org_companies, org_branches, and org_placements tables.
type Repository interface {
	Companies(ctx context.Context, activeOnly bool) ([]domain.Company, error)
	Company(ctx context.Context, id int64) (domain.Company, error)
	CreateCompany(ctx context.Context, c domain.Company) (int64, error)
	UpdateCompany(ctx context.Context, c domain.Company) error
	SetCompanyActive(ctx context.Context, id int64, active bool, at time.Time) error

	Branches(ctx context.Context, companyID int64) ([]domain.Branch, error)
	Branch(ctx context.Context, id int64) (domain.Branch, error)
	CreateBranch(ctx context.Context, b domain.Branch) (int64, error)
	UpdateBranch(ctx context.Context, b domain.Branch) error
	SetBranchActive(ctx context.Context, companyID, id int64, active bool, at time.Time) error

	PlacementsOfUser(ctx context.Context, userID int64) ([]UnitPlacement, error)
	// PlacesOfUser is every placement of the user, in active units or not.
	PlacesOfUser(ctx context.Context, userID int64) ([]domain.Placement, error)
	Placements(ctx context.Context, companyID int64) ([]domain.Placement, error)
	// AddPlacement reports false when the placement already exists.
	AddPlacement(ctx context.Context, p domain.Placement) (bool, error)
	// RemovePlacement reports false when there was none.
	RemovePlacement(ctx context.Context, userID, companyID, branchID int64) (bool, error)
	SetDefault(ctx context.Context, userID, companyID, branchID int64) (bool, error)
	PlacedUserIDs(ctx context.Context, companyID int64) ([]int64, error)
	IsPlaced(ctx context.Context, userID, companyID int64) (bool, error)
	CountPlacementsOfUser(ctx context.Context, userID int64) (int64, error)
	CountPlacements(ctx context.Context) (int64, error)
	// UsersWithAWorkplace lists users placed across an active company or in an active branch.
	UsersWithAWorkplace(ctx context.Context) ([]int64, error)
}

// Transactor runs fn in one transaction — the kernel's d.Tx.
type Transactor interface {
	Do(ctx context.Context, fn func(ctx context.Context) error, opts ...kernel.TxOption) error
}

// Publisher announces events — the kernel's d.Events.
type Publisher interface {
	Publish(ctx context.Context, name string, version int, payload any) error
}

// Auditor records changes — the kernel's d.Audit.
type Auditor interface {
	Record(ctx context.Context, e kernel.AuditEntry)
}

// Identity is who is signed in — the kernel's d.Auth.
type Identity interface {
	UserID(ctx context.Context) (int64, bool)
}

// Authorizer answers what the signed-in user may do where the request works — the kernel's
// d.Authz.
type Authorizer interface {
	Can(ctx context.Context, permission string) bool
	Units(ctx context.Context, permission string) (kernel.Units, error)
	// Covers reports whether the signed-in user may act on target's account (rbac: holds, where
	// it matters, everything target holds).
	Covers(ctx context.Context, target int64) bool
}

// Permissions this module declares (organization.module.go) and checks.
const (
	PermRead      = "organization.read"
	PermManage    = "organization.manage"
	PermCompanies = "organization.companies"
)

// Service is the organization module's use cases.
type Service struct {
	repo   Repository
	tx     Transactor
	users  ports.UserDirectory
	events Publisher
	audit  Auditor
	authz  Authorizer
	who    Identity
	clock  kernel.Clock

	mu    sync.Mutex
	zones map[string]*time.Location
}

func NewService(repo Repository, tx Transactor, users ports.UserDirectory, events Publisher, audit Auditor, authz Authorizer, who Identity, clock kernel.Clock) *Service {
	return &Service{repo: repo, tx: tx, users: users, events: events, audit: audit, authz: authz, who: who, clock: clock, zones: map[string]*time.Location{}}
}

func companyTarget(id int64) string { return fmt.Sprintf("company:%d", id) }
func branchTarget(id int64) string  { return fmt.Sprintf("branch:%d", id) }
func userTarget(id int64) string    { return fmt.Sprintf("user:%d", id) }

// here is the scope the request works in.
func here(ctx context.Context) kernel.Scope { return kernel.ScopeFrom(ctx) }

// --- companies ------------------------------------------------------------------------------

// CompanyInput is what creating or changing a company takes.
type CompanyInput struct {
	Code     string
	Name     string
	Timezone string
}

func (in CompanyInput) clean() domain.Company {
	return domain.Company{Code: domain.NormalizeCode(in.Code), Name: strings.TrimSpace(in.Name), Timezone: strings.TrimSpace(in.Timezone)}
}

// mayManageCompanies is the K1 rule: companies are managed from the home company as a whole,
// by someone holding organization.companies there — never from inside another company.
func (s *Service) mayManageCompanies(ctx context.Context) error {
	sc := here(ctx)
	if sc.CompanyID != domain.HomeCompany || sc.BranchID != 0 {
		return apperror.Forbidden("Perusahaan dikelola dari perusahaan induk (seluruh perusahaan) — pindah ke sana dulu")
	}
	if !s.authz.Can(ctx, PermCompanies) {
		return apperror.Forbidden("Anda tidak memiliki izin mengelola perusahaan")
	}
	return nil
}

// Companies lists every company, for whoever manages them.
func (s *Service) Companies(ctx context.Context) ([]domain.Company, error) {
	if err := s.mayManageCompanies(ctx); err != nil {
		return nil, err
	}
	return s.repo.Companies(ctx, false)
}

// CreateCompany adds a company. Its creator is placed across it and — through the
// organization.company.created event — becomes its Administrator (rbac).
func (s *Service) CreateCompany(ctx context.Context, actor int64, in CompanyInput) (domain.Company, error) {
	if err := s.mayManageCompanies(ctx); err != nil {
		return domain.Company{}, err
	}
	c := in.clean()
	var v validator.Validator
	domain.ValidateCompany(&v, c)
	if err := v.Err(); err != nil {
		return domain.Company{}, err
	}
	now := s.clock.Now()
	c.CreatedAt, c.UpdatedAt = now, now
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		id, err := s.repo.CreateCompany(ctx, c)
		if err != nil {
			return codeTaken(err)
		}
		c.ID = id
		if _, err := s.repo.AddPlacement(ctx, domain.Placement{UserID: actor, CompanyID: id, CreatedAt: now, CreatedBy: actor}); err != nil {
			return err
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "organization", Action: "company.created", Target: companyTarget(id), Summary: "Membuat perusahaan " + c.Name,
			Data: map[string]any{"code": c.Code, "name": c.Name, "timezone": c.Timezone}})
		return s.events.Publish(ctx, "organization.company.created", 1, companyCreated{CompanyID: id, Code: c.Code, Name: c.Name, CreatedBy: actor})
	})
	if err != nil {
		return domain.Company{}, err
	}
	c.IsActive = true
	return c, nil
}

// companyCreated is organization.company.created v1 (module.json emits; README).
type companyCreated struct {
	CompanyID int64  `json:"company_id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	CreatedBy int64  `json:"created_by"`
}

// UpdateCompany changes a company's code, name, or time zone.
func (s *Service) UpdateCompany(ctx context.Context, id int64, in CompanyInput) (domain.Company, error) {
	if err := s.mayManageCompanies(ctx); err != nil {
		return domain.Company{}, err
	}
	before, err := s.repo.Company(ctx, id)
	if err != nil {
		return domain.Company{}, notFound(err, "Perusahaan")
	}
	c := in.clean()
	var v validator.Validator
	domain.ValidateCompany(&v, c)
	if err := v.Err(); err != nil {
		return domain.Company{}, err
	}
	c.ID, c.IsActive, c.CreatedAt, c.UpdatedAt = id, before.IsActive, before.CreatedAt, s.clock.Now()
	if err := s.repo.UpdateCompany(ctx, c); err != nil {
		return domain.Company{}, codeTaken(notFound(err, "Perusahaan"))
	}
	s.audit.Record(ctx, kernel.AuditEntry{Module: "organization", Action: "company.updated", Target: companyTarget(id), Summary: "Mengubah perusahaan " + c.Name,
		Data: changes(map[string][2]string{"code": {before.Code, c.Code}, "name": {before.Name, c.Name}, "timezone": {before.Timezone, c.Timezone}})})
	return c, nil
}

// SetCompanyActive deactivates or reactivates a company. Deactivated, nobody works in it and
// its background work stops (d.Scope.Companies); its data stays. The home company stays active.
func (s *Service) SetCompanyActive(ctx context.Context, id int64, active bool) error {
	if err := s.mayManageCompanies(ctx); err != nil {
		return err
	}
	if id == domain.HomeCompany && !active {
		return apperror.Conflict("Perusahaan induk tidak dapat dinonaktifkan")
	}
	c, err := s.repo.Company(ctx, id)
	if err != nil {
		return notFound(err, "Perusahaan")
	}
	if err := s.repo.SetCompanyActive(ctx, id, active, s.clock.Now()); err != nil {
		return notFound(err, "Perusahaan")
	}
	action, summary := "company.deactivated", "Menonaktifkan perusahaan "+c.Name
	if active {
		action, summary = "company.activated", "Mengaktifkan perusahaan "+c.Name
	}
	s.audit.Record(ctx, kernel.AuditEntry{Kind: kernel.AuditSecurity, Module: "organization", Action: action, Target: companyTarget(id), Summary: summary})
	return nil
}

// --- branches -------------------------------------------------------------------------------

// BranchInput is what creating or changing a branch takes.
type BranchInput struct {
	Code     string
	Name     string
	Timezone string
	NITKU    string
	Address  string
}

func (in BranchInput) clean() domain.Branch {
	return domain.Branch{Code: domain.NormalizeCode(in.Code), Name: strings.TrimSpace(in.Name), Timezone: strings.TrimSpace(in.Timezone),
		NITKU: strings.TrimSpace(in.NITKU), Address: strings.TrimSpace(in.Address)}
}

// mayManageWholeCompany: branches, and placements across the company, need
// organization.manage across the whole company.
func (s *Service) mayManageWholeCompany(ctx context.Context) error {
	u, err := s.authz.Units(ctx, PermManage)
	if err != nil {
		return err
	}
	if !u.AllBranches || here(ctx).BranchID != 0 {
		return apperror.Forbidden("Perlu izin mengelola organisasi untuk seluruh perusahaan")
	}
	return nil
}

// Branches lists the branches of the company the request works in.
func (s *Service) Branches(ctx context.Context) ([]domain.Branch, error) {
	return s.repo.Branches(ctx, here(ctx).CompanyID)
}

func (s *Service) CreateBranch(ctx context.Context, in BranchInput) (domain.Branch, error) {
	if err := s.mayManageWholeCompany(ctx); err != nil {
		return domain.Branch{}, err
	}
	b := in.clean()
	var v validator.Validator
	domain.ValidateBranch(&v, b)
	if err := v.Err(); err != nil {
		return domain.Branch{}, err
	}
	now := s.clock.Now()
	b.CompanyID, b.IsActive, b.CreatedAt, b.UpdatedAt = here(ctx).CompanyID, true, now, now
	id, err := s.repo.CreateBranch(ctx, b)
	if err != nil {
		return domain.Branch{}, codeTaken(err)
	}
	b.ID = id
	s.audit.Record(ctx, kernel.AuditEntry{Module: "organization", Action: "branch.created", Target: branchTarget(id), Summary: "Membuat cabang " + b.Name,
		Data: map[string]any{"code": b.Code, "name": b.Name, "timezone": b.Timezone, "nitku": b.NITKU}})
	return b, nil
}

// branchHere is branch id, only when it belongs to the company the request works in.
func (s *Service) branchHere(ctx context.Context, id int64) (domain.Branch, error) {
	b, err := s.repo.Branch(ctx, id)
	if err != nil || b.CompanyID != here(ctx).CompanyID {
		return domain.Branch{}, notFound(ErrNotFound, "Cabang")
	}
	return b, nil
}

func (s *Service) UpdateBranch(ctx context.Context, id int64, in BranchInput) (domain.Branch, error) {
	if err := s.mayManageWholeCompany(ctx); err != nil {
		return domain.Branch{}, err
	}
	before, err := s.branchHere(ctx, id)
	if err != nil {
		return domain.Branch{}, err
	}
	b := in.clean()
	var v validator.Validator
	domain.ValidateBranch(&v, b)
	if err := v.Err(); err != nil {
		return domain.Branch{}, err
	}
	b.ID, b.CompanyID, b.IsActive, b.CreatedAt, b.UpdatedAt = id, before.CompanyID, before.IsActive, before.CreatedAt, s.clock.Now()
	if err := s.repo.UpdateBranch(ctx, b); err != nil {
		return domain.Branch{}, codeTaken(notFound(err, "Cabang"))
	}
	s.audit.Record(ctx, kernel.AuditEntry{Module: "organization", Action: "branch.updated", Target: branchTarget(id), Summary: "Mengubah cabang " + b.Name,
		Data: changes(map[string][2]string{"code": {before.Code, b.Code}, "name": {before.Name, b.Name}, "timezone": {before.Timezone, b.Timezone},
			"nitku": {before.NITKU, b.NITKU}, "address": {before.Address, b.Address}})})
	return b, nil
}

// SetBranchActive deactivates or reactivates a branch of the company the request works in.
func (s *Service) SetBranchActive(ctx context.Context, id int64, active bool) error {
	if err := s.mayManageWholeCompany(ctx); err != nil {
		return err
	}
	b, err := s.branchHere(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.SetBranchActive(ctx, b.CompanyID, id, active, s.clock.Now()); err != nil {
		return notFound(err, "Cabang")
	}
	action, summary := "branch.deactivated", "Menonaktifkan cabang "+b.Name
	if active {
		action, summary = "branch.activated", "Mengaktifkan cabang "+b.Name
	}
	s.audit.Record(ctx, kernel.AuditEntry{Kind: kernel.AuditSecurity, Module: "organization", Action: action, Target: branchTarget(id), Summary: summary})
	return nil
}

// --- helpers --------------------------------------------------------------------------------

// changes is the before/after of the fields that changed, for the audit record.
func changes(fields map[string][2]string) map[string]any {
	out := map[string]any{}
	for k, ba := range fields {
		if ba[0] != ba[1] {
			out[k] = map[string]any{"before": ba[0], "after": ba[1]}
		}
	}
	return out
}

func notFound(err error, what string) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound(what + " tidak ditemukan")
	}
	return err
}

func codeTaken(err error) error {
	if errors.Is(err, ErrCodeTaken) {
		return apperror.Validation([]validator.FieldError{{Field: "code", Message: "sudah dipakai"}})
	}
	return err
}
