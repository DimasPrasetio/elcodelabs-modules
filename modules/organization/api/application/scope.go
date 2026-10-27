package application

import (
	"context"
	"strconv"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/organization/domain"
	"__GO_MODULE__/internal/shared/apperror"
)

// Resolve answers the kernel (ScopeResolver) for every signed-in request: the scope the user
// asked for in X-Scope ("", "<company>", "<company>:<branch>"), if they are placed there. A
// company comes back only for someone placed across it; a branch for someone placed across its
// company or in it. Anything else — a deactivated company or branch, a malformed or unknown
// unit — is kernel.ErrScopeDenied, without saying which.
func (s *Service) Resolve(ctx context.Context, userID int64, requested string) (kernel.Scope, error) {
	company, branch, ok := parseUnit(requested)
	if !ok {
		return kernel.Scope{}, kernel.ErrScopeDenied
	}
	placements, err := s.repo.PlacementsOfUser(ctx, userID)
	if err != nil {
		return kernel.Scope{}, err
	}
	if requested == "" {
		p, ok := defaultPlacement(placements)
		if !ok {
			// Said plainly (the kernel passes an apperror through): signed in, placed nowhere.
			return kernel.Scope{}, errUnplaced
		}
		return s.scope(p.CompanyID, p.BranchID, p.CompanyTimezone, p.BranchTimezone), nil
	}
	var across *UnitPlacement
	for i, p := range placements {
		if p.CompanyID != company {
			continue
		}
		if p.BranchID == branch {
			return s.scope(company, branch, p.CompanyTimezone, p.BranchTimezone), nil
		}
		if p.BranchID == 0 {
			across = &placements[i]
		}
	}
	if across == nil || branch == 0 {
		return kernel.Scope{}, kernel.ErrScopeDenied
	}
	// Placed across the company: any of its active branches.
	b, err := s.repo.Branch(ctx, branch)
	if err != nil || b.CompanyID != company || !b.IsActive {
		return kernel.Scope{}, kernel.ErrScopeDenied
	}
	return s.scope(company, branch, across.CompanyTimezone, b.Timezone), nil
}

var errUnplaced = apperror.Forbidden("Akun Anda belum ditempatkan di perusahaan atau cabang mana pun. Hubungi administrator.")

// parseUnit reads "", "<company>", or "<company>:<branch>" with positive IDs.
func parseUnit(s string) (company, branch int64, ok bool) {
	if s == "" {
		return 0, 0, true
	}
	c, b, hasBranch := strings.Cut(s, ":")
	company, err := strconv.ParseInt(c, 10, 64)
	if err != nil || company < 1 {
		return 0, 0, false
	}
	if hasBranch {
		if branch, err = strconv.ParseInt(b, 10, 64); err != nil || branch < 1 {
			return 0, 0, false
		}
	}
	return company, branch, true
}

// defaultPlacement is the placement marked default; else the first across a company; else the
// first branch (placements come ordered by company, then branch).
func defaultPlacement(ps []UnitPlacement) (UnitPlacement, bool) {
	for _, p := range ps {
		if p.IsDefault {
			return p, true
		}
	}
	for _, p := range ps {
		if p.BranchID == 0 {
			return p, true
		}
	}
	if len(ps) > 0 {
		return ps[0], true
	}
	return UnitPlacement{}, false
}

// scope builds the kernel's scope; its time zone is the branch's, else the company's.
func (s *Service) scope(company, branch int64, companyTZ, branchTZ string) kernel.Scope {
	tz := branchTZ
	if tz == "" {
		tz = companyTZ
	}
	return kernel.Scope{CompanyID: company, BranchID: branch, Location: s.zone(tz)}
}

// zone loads a time zone once; an unknown one (never stored: validated) is the app's.
func (s *Service) zone(name string) *time.Location {
	s.mu.Lock()
	defer s.mu.Unlock()
	if loc, ok := s.zones[name]; ok {
		return loc
	}
	loc, err := time.LoadLocation(name)
	if err != nil || name == "" {
		loc = s.clock.Location()
	}
	s.zones[name] = loc
	return loc
}

// ActiveCompanies answers the kernel (ScopeResolver.Companies): the active companies, each in
// its time zone — what background work runs for.
func (s *Service) ActiveCompanies(ctx context.Context) ([]kernel.Scope, error) {
	cs, err := s.repo.Companies(ctx, true)
	if err != nil {
		return nil, err
	}
	out := make([]kernel.Scope, len(cs))
	for i, c := range cs {
		out[i] = s.scope(c.ID, 0, c.Timezone, "")
	}
	return out, nil
}

// Unit is one place a user may work, for the picker.
type Unit struct {
	CompanyID   int64
	CompanyCode string
	CompanyName string
	BranchID    int64 // 0: across the company
	BranchCode  string
	BranchName  string
	Placed      bool // a placement of the user's own (not a branch of a company placed across) — what can be the default
}

// Mine is where the signed-in user may work: every placement, and for one across a company,
// each of its active branches too — and which one is their default.
func (s *Service) Mine(ctx context.Context, userID int64) (units []Unit, def Unit, err error) {
	placements, err := s.repo.PlacementsOfUser(ctx, userID)
	if err != nil {
		return nil, Unit{}, err
	}
	seen := map[[2]int64]bool{}
	add := func(u Unit) {
		if !seen[[2]int64{u.CompanyID, u.BranchID}] {
			seen[[2]int64{u.CompanyID, u.BranchID}] = true
			units = append(units, u)
		}
	}
	for _, p := range placements {
		u := Unit{CompanyID: p.CompanyID, CompanyCode: p.CompanyCode, CompanyName: p.CompanyName, BranchID: p.BranchID, BranchCode: p.BranchCode, BranchName: p.BranchName, Placed: true}
		add(u)
		if p.BranchID == 0 {
			branches, err := s.repo.Branches(ctx, p.CompanyID)
			if err != nil {
				return nil, Unit{}, err
			}
			for _, b := range branches {
				if b.IsActive {
					add(Unit{CompanyID: p.CompanyID, CompanyCode: p.CompanyCode, CompanyName: p.CompanyName, BranchID: b.ID, BranchCode: b.Code, BranchName: b.Name})
				}
			}
		}
	}
	if p, ok := defaultPlacement(placements); ok {
		def = Unit{CompanyID: p.CompanyID, CompanyCode: p.CompanyCode, CompanyName: p.CompanyName, BranchID: p.BranchID, BranchCode: p.BranchCode, BranchName: p.BranchName, Placed: true}
	}
	return units, def, nil
}

// SetDefault makes one of the user's own placements their default.
func (s *Service) SetDefault(ctx context.Context, userID, company, branch int64) error {
	return s.tx.Do(ctx, func(ctx context.Context) error {
		ok, err := s.repo.SetDefault(ctx, userID, company, branch)
		if err != nil {
			return err
		}
		if !ok {
			return notFound(ErrNotFound, "Penempatan")
		}
		return nil
	})
}

// Company and the other reads below are the contract's (OrganizationClient).

func (s *Service) Company(ctx context.Context, id int64) (domain.Company, error) {
	return s.repo.Company(ctx, id)
}

func (s *Service) Branch(ctx context.Context, id int64) (domain.Branch, error) {
	return s.repo.Branch(ctx, id)
}

func (s *Service) BranchesOf(ctx context.Context, company int64) ([]domain.Branch, error) {
	return s.repo.Branches(ctx, company)
}

func (s *Service) PlacedUserIDs(ctx context.Context, company int64) ([]int64, error) {
	return s.repo.PlacedUserIDs(ctx, company)
}

func (s *Service) IsPlaced(ctx context.Context, userID, company int64) (bool, error) {
	return s.repo.IsPlaced(ctx, userID, company)
}

// Places is every placement of the user, in active units or not.
func (s *Service) Places(ctx context.Context, userID int64) ([]domain.Placement, error) {
	return s.repo.PlacesOfUser(ctx, userID)
}
